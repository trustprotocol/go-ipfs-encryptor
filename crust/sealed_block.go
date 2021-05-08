package crust

import (
	"encoding/json"
	"fmt"

	"github.com/ipfs/go-cid"
)

// Warp sealed block for putting, realize block.Block interface
type WarpedSealedBlock struct {
	cid  cid.Cid
	data []byte
}

func NewWarpedSealedBlock(path string, size int, c cid.Cid) *WarpedSealedBlock {
	bv, _ := json.Marshal(SealedBlock{Path: path, Size: size})
	return &WarpedSealedBlock{data: bv, cid: c}
}

func (b *WarpedSealedBlock) RawData() []byte {
	return b.data
}

func (b *WarpedSealedBlock) Cid() cid.Cid {
	return b.cid
}

func (b *WarpedSealedBlock) String() string {
	return fmt.Sprintf("[Block %s]", b.Cid())
}

func (b *WarpedSealedBlock) Loggable() map[string]interface{} {
	return map[string]interface{}{
		"block": b.Cid().String(),
	}
}

func IsWarpedSealedBlock(block interface{}) bool {
	switch block.(type) {
	case *WarpedSealedBlock:
		return true
	default:
		return false
	}
}

// Sealed block info
type SealedBlock struct {
	Path string `json:"path"`
	Size int    `json:"size"`
}

func TryGetSealedBlock(value []byte) (bool, *SealedBlock) {
	sb := &SealedBlock{}
	err := json.Unmarshal(value, sb)
	if err != nil {
		return false, nil
	}

	return true, sb
}

func (sb *SealedBlock) ToSealedInfo() *SealedInfo {
	return &SealedInfo{Sbs: []SealedBlock{*sb}}
}

// All sealed block info
type SealedInfo struct {
	Sbs []SealedBlock `json:"sbs"`
}

func (si *SealedInfo) Bytes() []byte {
	bs, _ := json.Marshal(si)
	return bs
}

func (si *SealedInfo) AddSealedBlock(sb SealedBlock) *SealedInfo {
	si.Sbs = append(si.Sbs, sb)
	return si
}

func TryGetSealedInfo(value []byte) (bool, *SealedInfo) {
	si := &SealedInfo{}
	err := json.Unmarshal(value, si)
	if err != nil {
		return false, nil
	}

	return true, si
}

func MergeSealedInfo(a *SealedInfo, b *SealedInfo) *SealedInfo {
	si := &SealedInfo{}
	si.Sbs = append(a.Sbs, b.Sbs...)
	return si
}
