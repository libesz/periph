package sysfs

import (
	"testing"
)

func TestAddrConvert(t *testing.T) {
	addrString := "28-04170328afff"
	resultNumber, _ := dirNameToAddress(addrString)
	result, _ := addressToDirName(resultNumber)
	if addrString != result {
		t.Fatal("Onewire address back-forth conversion failed")
	}
}

func TestTx(t *testing.T) {
	bus, err := NewOnewire(-1)
	if err != nil {
		t.Fatal("Bus open error:", err)
	}
	ww := make([]byte, 1, 90)
	ww[0] = 0x55
	//Random content
	ww = append(ww, []byte{0x28, 0x04, 0x17, 0x03, 0x28, 0xaf, 0xff, 0x12}...)
	r := make([]byte, 9, 9)
	err = bus.Tx(ww, r, false)
	if err := bus.Close(); err != nil {
		t.Fatal(err)
	}
}
