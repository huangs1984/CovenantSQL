/*
 * Copyright 2018 The ThunderDB Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package sqlchain

// TODO(leventeliu): use pooled objects to speed up this index.

import (
	"sync"

	"gitlab.com/thunderdb/ThunderDB/crypto/hash"
	ct "gitlab.com/thunderdb/ThunderDB/sqlchain/types"
	wt "gitlab.com/thunderdb/ThunderDB/worker/types"
)

var (
	placeHolder = &hash.Hash{}
)

// RequestTracker defines a tracker of a particular database query request.
// We use it to track and update queries in this index system.
type RequestTracker struct {
	// TODO(leventeliu): maybe we don't need them to be "signed" here. Given that the Response or
	// Ack is already verified, simply use Header.
	Response *wt.SignedResponseHeader
	Ack      *wt.SignedAckHeader

	// SignedBlock is the hash of the block in the currently best chain which contains this query.
	SignedBlock *hash.Hash
}

// QueryTracker defines a tracker of a particular database query. It may contain multiple queries
// to differe workers.
type QueryTracker struct {
	FirstAck *RequestTracker
	Queries  []*RequestTracker
}

// NewQuerySummary returns a new QueryTracker reference.
func NewQuerySummary() *QueryTracker {
	return &QueryTracker{
		// TODO(leventeliu): set appropriate capacity.
		FirstAck: nil,
		Queries:  make([]*RequestTracker, 0, 10),
	}
}

// UpdateAck updates the query tracker with a verified SignedAckHeader.
func (s *RequestTracker) UpdateAck(ack *wt.SignedAckHeader) (newAck bool, err error) {
	if s.Ack == nil {
		// A later Ack can overwrite the original Response setting
		*s = RequestTracker{
			Response: ack.SignedResponseHeader(),
			Ack:      ack,
		}

		newAck = true
	} else if !s.Ack.HeaderHash.IsEqual(&ack.HeaderHash) {
		// This may happen when a client sends multiple acknowledgements for a same query (same
		// response header hash)
		err = ErrMultipleAckOfResponse
	} // else it's same as s.Ack, let's try not to overwrite it

	return
}

// HashIndex defines a RequestTracker index using hash as key.
type HashIndex map[hash.Hash]*RequestTracker

// SeqIndex defines a QueryTracker index using sequence number as key.
type SeqIndex map[uint64]*QueryTracker

// Ensure returns the *QueryTracker associated with the given key. It creates a new item if the
// key doesn't exist.
func (i SeqIndex) Ensure(k uint64) (v *QueryTracker) {
	var ok bool

	if v, ok = i[k]; !ok {
		v = NewQuerySummary()
		i[k] = v
	}

	return
}

// MultiIndex defines a combination of multiple indexes.
//
// Index layout is as following:
//
//  RespIndex                                    +----------------+
//                +---------------------------+->| RequestTracker |       +---------------------------+
// |  ...   |     |                           |  | +-Response     |------>| SignedResponseHeader      |
// +--------+     |                           |  | +-Ack (nil)    |       | +-ResponseHeader          |
// | hash#1 |-----+                           |  | +-...          |       | | +-SignedRequestHeader   |
// +--------+                                 |  +----------------+       | | | +-RequestHeader       |
// |  ...   |                                 |                           | | | | +-...               |
// +--------+           +------------------+  |                           | | | | +-SeqNo: seq#0      |
// | hash#3 |-----+  +->| QueryTracker     |  |                           | | | | +-...               |
// +--------+     |  |  | +-FirstAck (nil) |  |                           | | | +-HeaderHash = hash#0 |
// |  ...   |     |  |  | +-Queries        |  |                           | | | +-Signee ====> pubk#0 |
// +--------+     |  |  |   +-[0]          |--+                           | | | +-Signature => sign#0 |
// | hash#6 |--+  |  |  |   +-...          |                              | | +-...                   |
// +--------+  |  |  |  +------------------+                              | +-HeaderHash = hash#1     |
// |  ...   |  |  |  |                                                    | +-Signee ====> pubk#1     |
//             |  |  |                                                    | +-Signature => sign#1     |
//             |  |  |                                                    +---------------------------+
//             |  |  |                           +----------------+
//             |  +-------------+---------+-+--->| RequestTracker |
//             |     |          |         | |    | +-Response     |----+  +-------------------------------+
//  AckIndex   |     |          |         | |    | +-Ack          |----|->| SignedAckHeader               |
//             |     |          |         | |    | +-...          |    |  | +-AckHeader                   |
// |  ...   |  |     |          |         | |    +----------------+    +->| | +-SignedResponseHeader      |
// +--------+  |     |          |         | |                             | | | +-ResponseHeader          |
// | hash#4 |--|----------------+         | |                             | | | | +-SignedRequestHeader   |
// +--------+  |     |                    | |                             | | | | | +-RequestHeader       |
// |  ...   |  |     |                    | |                             | | | | | | +-...               |
//             |     |                    | |                             | | | | | | +-SeqNo: seq#1      |
//             |     |                    | |                             | | | | | | +-...               |
//             |     |                    | |                             | | | | | +-HeaderHash = hash#2 |
//             |     |                    | |                             | | | | | +-Signee ====> pubk#2 |
//             |     |                    | |                             | | | | | +-Signature => sign#2 |
//  SeqIndex   |     |                    | |    +----------------+       | | | | +-...                   |
//             +------------------------------+->| RequestTracker |       | | | +-HeaderHash = hash#3     |
// |  ...   |        |                    | | |  | +-Response     |---+   | | | +-Signee ====> pubk#3     |
// +--------+        |                    | | |  | +-Ack (nil)    |   |   | | | +-Signature => sign#3     |
// | seq#0  |--------+                    | | |  | +-...          |   |   | | +-...                       |
// +--------+                             | | |  +----------------+   |   | +-HeaderHash = hash#4         |
// |  ...   |                             | | |                       |   | +-Signee ====> pubk#2         |
// +--------+           +--------------+  | | |                       |   | +-Signature => sign#4         |
// | seq#1  |---------->| QueryTracker |  | | |                       |   +-------------------------------+
// +--------+           | +-FirstAck   |--+ | |                       |
// |  ...   |           | +-Queries    |    | |                       |
//                      |   +-[0]      |----+ |                       |
//                      |   +-[1]      |------+                       |   +---------------------------+
//                      |   +-...      |                              +-->| SignedResponseHeader      |
//                      +--------------+                                  | +-ResponseHeader          |
//                                                                        | | +-SignedRequestHeader   |
//                                                                        | | | +-RequestHeader       |
//                                                                        | | | | +-...               |
//                                                                        | | | | +-SeqNo: seq#1      |
//                                                                        | | | | +-...               |
//                                                                        | | | +-HeaderHash = hash#5 |
//                                                                        | | | +-Signee ====> pubk#5 |
//                                                                        | | | +-Signature => sign#5 |
//                                                                        | | +-...                   |
//                                                                        | +-HeaderHash = hash#6     |
//                                                                        | +-Signee ====> pubk#6     |
//                                                                        | +-Signature => sign#6     |
//                                                                        +---------------------------+
//
type MultiIndex struct {
	sync.Mutex
	RespIndex, AckIndex HashIndex
	SeqIndex
}

// NewMultiIndex returns a new MultiIndex reference.
func NewMultiIndex() *MultiIndex {
	return &MultiIndex{
		RespIndex: make(map[hash.Hash]*RequestTracker),
		AckIndex:  make(map[hash.Hash]*RequestTracker),
		SeqIndex:  make(map[uint64]*QueryTracker),
	}
}

// AddResponse adds the responsed query to the index.
func (i *MultiIndex) AddResponse(resp *wt.SignedResponseHeader) (err error) {
	i.Lock()
	defer i.Unlock()

	if v, ok := i.RespIndex[resp.HeaderHash]; ok {
		if v == nil || v.Response == nil {
			// TODO(leventeliu): consider to panic.
			err = ErrCorruptedIndex
			return
		}

		// Given that `resp` is already verified by user, its header should be deeply equal to
		// v.Response.ResponseHeader.
		// Considering that we may allow a node to update its key pair on-the-fly, just overwrite
		// this Response.
		v.Response = resp
		return
	}

	// Create new item
	s := &RequestTracker{
		Response: resp,
	}

	i.RespIndex[resp.HeaderHash] = s
	q := i.SeqIndex.Ensure(resp.Request.SeqNo)
	q.Queries = append(q.Queries, s)

	return nil
}

// AddAck adds the acknowledged query to the index.
func (i *MultiIndex) AddAck(ack *wt.SignedAckHeader) (err error) {
	i.Lock()
	defer i.Unlock()
	var v *RequestTracker
	var ok bool
	q := i.SeqIndex.Ensure(ack.SignedRequestHeader().SeqNo)

	if v, ok = i.RespIndex[ack.ResponseHeaderHash()]; ok {
		if v == nil || v.Response == nil {
			// TODO(leventeliu): consider to panic.
			err = ErrCorruptedIndex
			return
		}

		// Add hash -> ack index anyway, so that we can find the request tracker later, even if
		// there is a earlier acknowledgement for the same request
		i.AckIndex[ack.HeaderHash] = v

		// This also updates the item indexed by AckIndex and SeqIndex
		var newAck bool

		if newAck, err = v.UpdateAck(ack); err != nil {
			return
		}

		if newAck {
			q.Queries = append(q.Queries, v)
		}
	} else {
		// Build new QueryTracker and update both indexes
		v = &RequestTracker{
			Response: ack.SignedResponseHeader(),
			Ack:      ack,
		}

		i.RespIndex[ack.ResponseHeaderHash()] = v
		i.AckIndex[ack.HeaderHash] = v
		q.Queries = append(q.Queries, v)
	}

	// TODO(leventeliu):
	// This query has multiple signed acknowledgements. It may be caused by a network problem.
	// We will keep the first ack counted anyway. But, should we report it to someone?
	if q.FirstAck == nil {
		q.FirstAck = v
	} else if !q.FirstAck.Ack.HeaderHash.IsEqual(&ack.HeaderHash) {
		err = ErrMultipleAckOfSeqNo
	}

	return
}

// SetSignedBlock sets the signed block of the acknowledged query.
func (i *MultiIndex) SetSignedBlock(blockHash *hash.Hash, ackHeaderHash *hash.Hash) {
	i.Lock()
	defer i.Unlock()

	if v, ok := i.AckIndex[*ackHeaderHash]; ok {
		v.SignedBlock = blockHash
	}
}

// CheckBeforeExpire checks the index and does some necessary work before it expires.
func (i *MultiIndex) CheckBeforeExpire() {
	i.Lock()
	defer i.Unlock()

	for _, q := range i.SeqIndex {
		if ack := q.FirstAck; ack == nil {
			// TODO(leventeliu):
			// This query is not acknowledged and expires now.
		} else if ack.SignedBlock == nil || ack.SignedBlock == placeHolder {
			// TODO(leventeliu):
			// This query was acknowledged normally but collectors didn't pack it in any block.
			// There is definitely something wrong with them.
		}

		for _, s := range q.Queries {
			if s != q.FirstAck {
				// TODO(leventeliu): so these guys lost the competition in this query. Should we
				// do something about it?
			}
		}
	}
}

// CheckAckFromBlock checks a acknowledged query from a block in this index.
func (i *MultiIndex) CheckAckFromBlock(b *hash.Hash, ack *hash.Hash) (isKnown bool, err error) {
	i.Lock()
	defer i.Unlock()

	// Check acknowledgement
	q, isKnown := i.AckIndex[*ack]

	if !isKnown {
		return
	}

	if q.SignedBlock != nil && !q.SignedBlock.IsEqual(b) {
		err = ErrQuerySignedByAnotherBlock
		return
	}

	qs := i.SeqIndex[q.Ack.SignedRequestHeader().SeqNo]

	// Check it as a first acknowledgement
	if i.RespIndex[q.Response.HeaderHash] != q || qs == nil || qs.FirstAck == nil {
		err = ErrCorruptedIndex
		return
	}

	// If `q` is not considered first acknowledgement of this query locally
	if qs.FirstAck != q {
		if qs.FirstAck.SignedBlock != nil {
			err = ErrQuerySignedByAnotherBlock
			return
		}

		// But if the acknowledgement is not signed yet, it is also acceptable to promote another
		// acknowledgement
		qs.FirstAck = q
	}

	return
}

// MarkAndCollectUnsignedAcks marks and collects all the unsigned acknowledgements in the index.
func (i *MultiIndex) MarkAndCollectUnsignedAcks(qs *[]*hash.Hash) {
	i.Lock()
	defer i.Unlock()

	for _, q := range i.SeqIndex {
		if ack := q.FirstAck; ack != nil && ack.SignedBlock == nil {
			ack.SignedBlock = placeHolder
			*qs = append(*qs, &ack.Ack.HeaderHash)
		}
	}
}

// HeightIndex defines a MultiIndex index using height as key.
type HeightIndex map[int32]*MultiIndex

// EnsureHeight returns the *MultiIndex associated with the given height. It creates a new item if
// the key doesn't exist.
func (i HeightIndex) EnsureHeight(h int32) (v *MultiIndex) {
	v, ok := i[h]

	if !ok {
		v = NewMultiIndex()
		i[h] = v
	}

	return
}

// EnsureRange creates new *MultiIndex items associated within the given height range [l, h) for
// those don't exist.
func (i HeightIndex) EnsureRange(l, h int32) {
	for x := l; x < h; x++ {
		if _, ok := i[x]; !ok {
			i[x] = NewMultiIndex()
		}
	}
}

// QueryIndex defines a query index maintainer.
type QueryIndex struct {
	barrier     int32
	heightIndex HeightIndex
}

// NewQueryIndex returns a new QueryIndex reference.
func NewQueryIndex() *QueryIndex {
	return &QueryIndex{
		heightIndex: make(map[int32]*MultiIndex),
	}
}

// AddResponse adds the responsed query to the index.
func (i *QueryIndex) AddResponse(h int32, resp *wt.SignedResponseHeader) error {
	// TODO(leventeliu): we should ensure that the Request uses coordinated timestamp, instead of
	// any client local time.
	return i.heightIndex.EnsureHeight(h).AddResponse(resp)
}

// AddAck adds the acknowledged query to the index.
func (i *QueryIndex) AddAck(h int32, ack *wt.SignedAckHeader) error {
	return i.heightIndex.EnsureHeight(h).AddAck(ack)
}

// CheckAckFromBlock checks a acknowledged query from a block at the given height.
func (i *QueryIndex) CheckAckFromBlock(h int32, b *hash.Hash, ack *hash.Hash) (bool, error) {
	if h < i.barrier {
		return false, ErrQueryExpired
	}

	return i.heightIndex.EnsureHeight(h).CheckAckFromBlock(b, ack)
}

// SetSignedBlock updates the signed block in index for the acknowledged queries in the block.
func (i *QueryIndex) SetSignedBlock(h int32, b *ct.Block) {
	hi := i.heightIndex.EnsureHeight(h)

	for _, v := range b.Queries {
		hi.SetSignedBlock(&b.SignedHeader.BlockHash, v)
	}
}

// GetAck gets the acknowledged queries from the index.
func (i *QueryIndex) GetAck(h int32, header *hash.Hash) (
	ack *wt.SignedAckHeader, err error,
) {
	if h >= i.barrier {
		if q, ok := i.heightIndex.EnsureHeight(h).AckIndex[*header]; ok {
			ack = q.Ack
		} else {
			err = ErrQueryNotCached
		}
	} else {
		err = ErrQueryExpired
	}

	return
}

// AdvanceBarrier moves barrier to given height. All buckets lower than this height will be set as
// expired, and all the queries which are not packed in these buckets will be reported.
func (i *QueryIndex) AdvanceBarrier(height int32) {
	for x := i.barrier; x < height; x++ {
		if hi, ok := i.heightIndex[x]; ok {
			hi.CheckBeforeExpire()
			delete(i.heightIndex, x)
		}
	}

	i.barrier = height
}

// MarkAndCollectUnsignedAcks marks and collects all the unsigned acknowledgements which can be
// signed by a block at the given height.
func (i *QueryIndex) MarkAndCollectUnsignedAcks(height int32) (qs []*hash.Hash) {
	qs = make([]*hash.Hash, 0, 1024)

	for x := i.barrier; x < height; x++ {
		if hi, ok := i.heightIndex[x]; ok {
			hi.MarkAndCollectUnsignedAcks(&qs)
		}
	}

	return
}
