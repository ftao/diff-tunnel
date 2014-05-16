package main

import (
	"bytes"
	"github.com/kr/binarydist"
)

func MakeDiff(old []byte, new_ []byte) []byte {
	oldR := bytes.NewBuffer(old)
	newR := bytes.NewBuffer(new_)
	patch := new(bytes.Buffer)
	binarydist.Diff(oldR, newR, patch)
	return patch.Bytes()
}

func Patch(old []byte, patch []byte) []byte {
	oldR := bytes.NewBuffer(old)
	patchR := bytes.NewBuffer(patch)
	newR := new(bytes.Buffer)
	binarydist.Patch(oldR, newR, patchR)
	return newR.Bytes()
}

type DiffContent struct {
	CacheKey []byte
	PatchTo  []byte
	Diff     []byte
}
