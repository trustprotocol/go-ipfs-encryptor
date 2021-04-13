package crust

import (
	"context"

	"github.com/crustio/go-ipfs-encryptor/utils"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
)

var sealBlackSet map[cid.Cid]bool
var sealBlackList = []string{
	"QmQPeNsJPyVWPFDVHb77w8G42Fvo15z4bG2X8D2GhfbSXc",
	"QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn",
}

func init() {
	sealBlackSet = make(map[cid.Cid]bool)
	for _, v := range sealBlackList {
		c, _ := cid.Decode(v)
		sealBlackSet[c] = true
	}
}

func startSeal(root cid.Cid, value []byte, sessionKey string) (returnInfo, *SealedBlock) {
	canSeal, path, err := sw.seal(root, sessionKey, false, value)
	if err != nil || !canSeal {
		return returnInfo{CanSeal: canSeal, Err: err}, nil
	}

	sb := &SealedBlock{
		Path: path,
		Size: len(value),
	}

	return returnInfo{CanSeal: true, Err: nil}, sb
}

func sealBlockAsync(root cid.Cid, leaf cid.Cid, value []byte, sessionKey string, serialMap *serialMap, lpool *utils.Lpool) {
	canSeal, path, err := sw.seal(root, sessionKey, false, value)
	if err != nil || !canSeal {
		serialMap.Rinfo <- returnInfo{CanSeal: canSeal, Err: err}
		lpool.Done()
		return
	}

	sb := SealedBlock{
		Path: path,
		Size: len(value),
	}

	serialMap.add(root, sb)
	lpool.Done()
}

func endSeal(root cid.Cid, sessionKey string) returnInfo {
	canSeal, _, err := sw.seal(root, sessionKey, false, []byte{})
	return returnInfo{CanSeal: canSeal, Err: err}
}

func deepSeal(ctx context.Context, originRootCid cid.Cid, rootNode ipld.Node, serv ipld.DAGService, sessionKey string, sealedMap *serialMap, lpool *utils.Lpool) returnInfo {
	for i := 0; i < len(rootNode.Links()); i++ {
		select {
		case rinfo := <-sealedMap.Rinfo:
			return rinfo
		default:
		}

		leafNode, err := serv.Get(ctx, rootNode.Links()[i].Cid)
		if err != nil {
			return returnInfo{CanSeal: false, Err: err}
		}

		rinfo := deepSeal(ctx, originRootCid, leafNode, serv, sessionKey, sealedMap, lpool)
		if rinfo.Err != nil || !rinfo.CanSeal {
			return rinfo
		}

		lpool.Add(1)
		go sealBlockAsync(originRootCid, leafNode.Cid(), leafNode.RawData(), sessionKey, sealedMap, lpool)
	}

	return returnInfo{CanSeal: true, Err: nil}
}

func Seal(ctx context.Context, root cid.Cid, serv ipld.DAGService) (bool, map[cid.Cid]SealedBlock, error) {
	// Black list
	if _, ok := sealBlackSet[root]; ok {
		return false, nil, nil
	}

	// Base data
	sealedMap := newSerialMap()
	sessionKey := utils.RandStringRunes(8)

	// Start seal root block
	rootNode, err := serv.Get(ctx, root)
	if err != nil {
		return false, nil, err
	}

	rinfo, sb := startSeal(rootNode.Cid(), rootNode.RawData(), sessionKey)
	if rinfo.Err != nil || !rinfo.CanSeal {
		return rinfo.CanSeal, nil, rinfo.Err
	}
	sealedMap.Data[rootNode.Cid()] = *sb

	// Multi-threaded deep seal
	lpool := utils.NewLpool(4)
	rinfo = deepSeal(ctx, rootNode.Cid(), rootNode, serv, sessionKey, sealedMap, lpool)
	if rinfo.Err != nil || !rinfo.CanSeal {
		return rinfo.CanSeal, nil, rinfo.Err
	}

	lpool.Wait()
	select {
	case rinfo := <-sealedMap.Rinfo:
		return rinfo.CanSeal, nil, rinfo.Err
	default:
	}

	// End seal
	rinfo = endSeal(root, sessionKey)

	return rinfo.CanSeal, sealedMap.Data, rinfo.Err
}

type returnInfo struct {
	CanSeal bool
	Err     error
}

// Convert map to serial map
type serialMap struct {
	Data  map[cid.Cid]SealedBlock
	ch    chan func()
	Rinfo chan returnInfo
}

func newSerialMap() *serialMap {
	m := &serialMap{
		Data: make(map[cid.Cid]SealedBlock),
		ch:   make(chan func()),
	}
	go func() {
		for {
			(<-m.ch)()
		}
	}()
	return m
}

func (m *serialMap) add(key cid.Cid, sb SealedBlock) {
	m.ch <- func() {
		m.Data[key] = sb
	}
}
