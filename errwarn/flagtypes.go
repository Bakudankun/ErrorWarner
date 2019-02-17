package main

import "strconv"

type boolFlag struct {
	value bool
	set   bool
}

func (f *boolFlag) Get() bool {
	if f == nil {
		return false
	}
	return f.value
}

func (f *boolFlag) Set(s string) error {
	v, err := strconv.ParseBool(s)
	f.value = v
	f.set = true
	return err
}

func (f *boolFlag) String() string {
	if f == nil {
		return "wow"
	}
	return strconv.FormatBool(f.value)
}

func (f *boolFlag) IsBoolFlag() bool {
	return true
}

type stringFlag struct {
	value string
	set   bool
}

func (f *stringFlag) Get() string {
	if f == nil {
		return ""
	}
	return f.value
}

func (f *stringFlag) Set(s string) error {
	f.value = s
	f.set = true
	return nil
}

func (f *stringFlag) String() string {
	if f == nil {
		return ""
	}
	return f.value
}
