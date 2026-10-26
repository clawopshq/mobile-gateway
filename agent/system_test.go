package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fakeBluetoothctl 은 PATH 앞에 가짜 bluetoothctl 을 둔다. 출력·종료코드는 BlueZ 실측값을 흉내낸다.
func fakeBluetoothctl(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bluetoothctl"), []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// BlueZ 5.66+(Debian 12·Ubuntu 24.04): `devices Paired` 만 있고 `paired-devices` 는 없다.
const bluez572 = `case "$*" in
  "devices Paired") echo "Device AA:BB:CC:DD:EE:01 Paired Phone" ;;
  "devices") echo "Device AA:BB:CC:DD:EE:01 Paired Phone"; echo "Device AA:BB:CC:DD:EE:02 Nearby Speaker" ;;
  *) echo "Invalid command in menu main: $1"; exit 1 ;;
esac
`

// BlueZ 5.64(Ubuntu 22.04): `paired-devices` 만 있고 `devices` 는 필터를 받지 않는다.
const bluez564 = `case "$*" in
  "paired-devices") echo "Device AA:BB:CC:DD:EE:01 Paired Phone" ;;
  "devices") echo "Device AA:BB:CC:DD:EE:01 Paired Phone"; echo "Device AA:BB:CC:DD:EE:02 Nearby Speaker" ;;
  "devices Paired") echo "Too many arguments"; exit 1 ;;
  *) echo "Invalid command in menu main: $1"; exit 1 ;;
esac
`

func TestBTDevices_PairedOnly(t *testing.T) {
	for _, tc := range []struct{ name, script string }{
		{"BlueZ 5.72", bluez572},
		{"BlueZ 5.64", bluez564},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fakeBluetoothctl(t, tc.script)
			got := (&System{}).btDevices(true)
			if len(got) != 1 || got[0].MAC != "AA:BB:CC:DD:EE:01" || got[0].Name != "Paired Phone" || !got[0].Paired {
				t.Errorf("페어링 목록 = %+v, 페어링된 폰 1대여야 한다", got)
			}
		})
	}
}

func TestBTDevices_All(t *testing.T) {
	fakeBluetoothctl(t, bluez572)
	got := (&System{}).btDevices(false)
	if len(got) != 2 || got[0].Paired || got[1].Paired {
		t.Errorf("전체 목록 = %+v, 2대·Paired=false 여야 한다", got)
	}
}

func TestRunTimed_ReportsTimeout(t *testing.T) {
	fakeBluetoothctl(t, `exec sleep 5`) // exec: sh 만 죽으면 자식 sleep 이 파이프를 잡아 5초를 다 기다린다
	if _, ok, timedOut := runTimed(200*time.Millisecond, "bluetoothctl", "devices", "Paired"); ok || !timedOut {
		t.Errorf("ok=%v timedOut=%v, 시간 초과로 보고해야 한다", ok, timedOut)
	}
	fakeBluetoothctl(t, bluez564)
	if _, ok, timedOut := runTimed(5*time.Second, "bluetoothctl", "devices", "Paired"); ok || timedOut {
		t.Errorf("ok=%v timedOut=%v, 문법 오류(exit 1)는 시간 초과가 아니다", ok, timedOut)
	}
}

func TestBTDevices_BothSyntaxesFail(t *testing.T) {
	fakeBluetoothctl(t, `echo "Waiting to connect to bluetoothd..."; exit 1`)
	if got := (&System{}).btDevices(true); got != nil {
		t.Errorf("둘 다 실패하면 nil 이어야 한다: %+v", got)
	}
}
