// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package storecache

import (
	"context"
	"encoding/base64"
	"strconv"

	"github.com/oklog/ulid"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"golang.org/x/crypto/blake2b"
)

const (
	cacheTypePostings string = "Postings"
	cacheTypeSeries   string = "Series"

	sliceHeaderSize = 16
)

var (
	ulidSize = uint64(len(ulid.ULID{}))
)

// IndexCache is the interface exported by index cache backends.
// Store operations do not support context.Context, deadlines need to be
// supported by the backends themselves. This is because Set operations are
// run async and it does not make sense to attach same context
// (potentially with a deadline) as in the original user's request.
type IndexCache interface {
	// StorePostings stores postings for a single series.
	StorePostings(blockID ulid.ULID, l labels.Label, v []byte)

	// FetchMultiPostings fetches multiple postings - each identified by a label -
	// and returns a map containing cache hits, along with a list of missing keys.
	FetchMultiPostings(ctx context.Context, blockID ulid.ULID, keys []labels.Label) (hits map[labels.Label][]byte, misses []labels.Label)

	// StoreSeries stores a single series.
	StoreSeries(blockID ulid.ULID, id storage.SeriesRef, v []byte)

	// FetchMultiSeries fetches multiple series - each identified by ID - from the cache
	// and returns a map containing cache hits, along with a list of missing IDs.
	FetchMultiSeries(ctx context.Context, blockID ulid.ULID, ids []storage.SeriesRef) (hits map[storage.SeriesRef][]byte, misses []storage.SeriesRef)
}

type cacheKey struct {
	block ulid.ULID
	key   interface{}
}

func (c cacheKey) keyType() string {
	switch c.key.(type) {
	case cacheKeyPostings:
		return cacheTypePostings
	case cacheKeySeries:
		return cacheTypeSeries
	}
	return "<unknown>"
}

func (c cacheKey) size() uint64 {
	switch k := c.key.(type) {
	case cacheKeyPostings:
		// ULID + 2 slice headers + number of chars in value and name.
		return ulidSize + 2*sliceHeaderSize + uint64(len(k.Value)+len(k.Name))
	case cacheKeySeries:
		return ulidSize + 8 // ULID + uint64.
	}
	return 0
}

func (c cacheKey) string() string {
	switch c.key.(type) {
	case cacheKeyPostings:
		// Use cryptographically hash functions to avoid hash collisions
		// which would end up in wrong query results.
		lbl := c.key.(cacheKeyPostings)
		lblHash := blake2b.Sum256([]byte(lbl.Name + ":" + lbl.Value))
		return "P:" + c.block.String() + ":" + base64.RawURLEncoding.EncodeToString(lblHash[0:])
	case cacheKeySeries:
		return "S:" + c.block.String() + ":" + strconv.FormatUint(uint64(c.key.(cacheKeySeries)), 10)
	default:
		return ""
	}
}

type cacheKeyPostings labels.Label
type cacheKeySeries uint64
