// Copyright 2016 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package sysfs

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"periph.io/x/periph"
	"periph.io/x/periph/conn/onewire"
	"periph.io/x/periph/conn/onewire/onewirereg"
)

// Onewire represents a w1 bus
type Onewire struct {
	busNumber int
}

// NewOnewire is called from Init()
func NewOnewire(busNumber int) (*Onewire, error) {
	if isLinux {
		return newOnewire(busNumber)
	}
	return nil, errors.New("sysfs-onewire: is not supported on this platform")
}

func newOnewire(busNumber int) (*Onewire, error) {
	return &Onewire{busNumber: busNumber}, nil
}

const w1MastersPrefix = "/sys/devices/w1_bus_master"

func (o *Onewire) Tx(w, r []byte, power onewire.Pullup) error {
	if len(w) < 9 || w[0] != 0x55 {
		return fmt.Errorf("not a valid device selection")
	}
	//Set pullup
	pullupPath := w1MastersPrefix + strconv.Itoa(o.busNumber) + "/w1_master_pullup"
	var toPullupFile byte = '1'
	if power == onewire.StrongPullup {
		toPullupFile = '0'
	}
	f, err := os.OpenFile(pullupPath, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	n, err := f.Write([]byte{toPullupFile})
	if n < 1 {
		return fmt.Errorf("pullup write incomplete")
	}
	if err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	//Write w to device rw interface
	deviceDir, _ := addressToDirName(onewire.Address(binary.LittleEndian.Uint64(w[1:9])))
	endPointPath := w1MastersPrefix + strconv.Itoa(o.busNumber) + "/" + deviceDir + "/rw"
	f, err = os.OpenFile(endPointPath, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	n, err = f.Write(w[9:])
	if n < len(w[9:]) {
		return fmt.Errorf("write incomplete")
	}
	// fmt.Println("device <- *", w[9:], "*")
	if err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	//Read the device buffer to r
	f, err = os.OpenFile(endPointPath, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	_, err = f.Read(r)
	// fmt.Println("device -> *", r, "*")
	if err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return nil
}

//TODO: alarmOnly is ignored
func (o *Onewire) Search(alarmOnly bool) ([]onewire.Address, error) {
	patternForParent := w1MastersPrefix + strconv.Itoa(o.busNumber) + "/"
	items, err := filepath.Glob(patternForParent + "*-*")
	if len(items) == 0 {
		return nil, errors.New("no onewire device found in sysfs")
	}
	if err != nil {
		return nil, err
	}
	var devices []onewire.Address
	for _, item := range items {
		addressStr := item[len(patternForParent):]
		address, err := dirNameToAddress(addressStr)
		if err != nil {
			return nil, err
		}
		devices = append(devices, onewire.Address(address))
	}
	return devices, nil
}

// addressToDirName converts onewire.Address (uint64) to
// device directory name as linux creates
// 0xCR04170328afff28 -> "28-04170328afff"
func addressToDirName(a onewire.Address) (string, error) {
	dump := fmt.Sprintf("%016x", a)
	family := dump[len(dump)-2:]
	deviceID := dump[2 : len(dump)-2]
	result := family + "-" + deviceID

	return result, nil
}

// dirNameToAddress converts the directory name which represents the
// device to device address as onewire.Address (uint64)
// "28-04170328afff" -> 0xCR04170328afff28
func dirNameToAddress(s string) (onewire.Address, error) {
	family, err := hex.DecodeString(s[0:2])
	if err != nil {
		return 0, errors.New("sysfs onewire device family decode error")
	}

	idBytes, err := hex.DecodeString(s[3:])
	if err != nil {
		return 0, errors.New("sysfs onewire device address decode error")
	}
	idBytes = append(idBytes, family[0])
	idBytes = append([]byte{onewire.CalcCRC(idBytes)}, idBytes...)

	result := uint64(0)
	for _, value := range idBytes {
		result = result << 8
		result += uint64(value)
	}
	return onewire.Address(result), nil
}

// Close satisfies BusCloser
func (o *Onewire) Close() error {
	return nil
}

// driverOnewire implements periph.Driver.
type driverOnewire struct {
	buses []string
}

func (d *driverOnewire) String() string {
	return "sysfs-onewire"
}

func (d *driverOnewire) Prerequisites() []string {
	return nil
}

func (d *driverOnewire) Init() (bool, error) {
	items, err := filepath.Glob(w1MastersPrefix + "*")
	if len(items) == 0 {
		return false, errors.New("no onewire bus found in sysfs")
	}
	if err != nil {
		return true, err
	}
	for _, item := range items {
		bus, err := strconv.Atoi(item[len(w1MastersPrefix):])
		if err != nil {
			continue
		}
		if err := onewirereg.Register("sysfs-"+item[len(w1MastersPrefix):], nil, bus, openerOnewire(bus).Open); err != nil {
			return true, err
		}
	}
	return true, nil
}

type openerOnewire int

func (o openerOnewire) Open() (onewire.BusCloser, error) {
	b, err := NewOnewire(int(o))
	if err != nil {
		return nil, err
	}
	return b, nil
}

func init() {
	if isLinux {
		periph.MustRegister(&driverOnewire{})
	}
}
