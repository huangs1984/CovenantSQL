/*
 * Copyright 2018 The CovenantSQL Authors.
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

package types

import (
	"encoding"
	"reflect"
	"testing"

	"bytes"

	"github.com/CovenantSQL/CovenantSQL/utils"
	. "github.com/smartystreets/goconvey/convey"
)

func TestHeader_MarshalUnmarshalBinary(t *testing.T) {

	block, err := generateRandomBlock(genesisHash, false)
	header := &block.SignedHeader.Header
	if err != nil {
		t.Fatalf("Failed to generate block: %v", err)
	}

	enc, err := utils.EncodeMsgPack(header)
	if err != nil {
		t.Fatalf("Failed to mashal binary: %v", err)
	}

	dec := &Header{}
	err = utils.DecodeMsgPack(enc.Bytes(), dec)
	if err != nil {
		t.Fatalf("Failed to unmashal binary: %v", err)
	}

	if !reflect.DeepEqual(header, dec) {
		t.Fatalf("Value not math:\n\tv1 = %+v\n\tv2 = %+v", block, dec)
	}
}

func TestSignedHeader_MarshalUnmashalBinary(t *testing.T) {
	block, err := generateRandomBlock(genesisHash, false)
	signedHeader := &block.SignedHeader
	if err != nil {
		t.Fatalf("Failed to generate block: %v", err)
	}

	enc, err := utils.EncodeMsgPack(signedHeader)
	if err != nil {
		t.Fatalf("Failed to mashal binary: %v", err)
	}

	dec := &SignedHeader{}
	err = utils.DecodeMsgPack(enc.Bytes(), dec)
	if err != nil {
		t.Fatalf("Failed to unmashal binary: %v", err)
	}

	if !reflect.DeepEqual(signedHeader, dec) {
		t.Fatalf("Value not math:\n\tv1 = %+v\n\tv2 = %+v", signedHeader, dec)
	}

}

func TestBlock_MarshalUnmarshalBinary(t *testing.T) {
	block, err := generateRandomBlock(genesisHash, false)
	if err != nil {
		t.Fatalf("Failed to generate block: %v", err)
	}
	h := reflect.TypeOf(block)
	_, ok := h.(encoding.BinaryMarshaler)
	if ok {
		t.Log("dec hash BinaryMashaler interface")
	}

	enc, err := block.Serialize()
	if err != nil {
		t.Fatalf("Failed to mashal binary: %v", err)
	}

	dec := &Block{}

	err = dec.Deserialize(enc)
	if err != nil {
		t.Fatalf("Failed to unmashal binary: %v", err)
	}

	bts1, err := block.MarshalHash()
	if err != nil {
		t.Fatal(err)
	}
	bts2, err := dec.MarshalHash()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bts1, bts2) {
		t.Fatal("hash not stable")
	}

	if !reflect.DeepEqual(block, dec) {
		t.Fatalf("value not match")
	}
}

func TestBlock_PackAndSignBlock(t *testing.T) {
	block, err := generateRandomBlock(genesisHash, false)
	if err != nil {
		t.Fatalf("Failed to generate block: %v", err)
	}

	err = block.Verify()
	if err != nil {
		t.Fatalf("Failed to verify: %v", err)
	}

	block.SignedHeader.BlockHash[0]++
	err = block.Verify()
	if err != ErrHashVerification {
		t.Fatalf("Unexpected error: %v", err)
	}

	tb, err := generateRandomTxBilling()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	block.PushTx(tb)
	err = block.Verify()
	if err != ErrMerkleRootVerification {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestOther_MarshalHash(t *testing.T) {
	Convey("marshal hash", t, func() {
		tm := TxType(1)
		s, err := tm.MarshalHash()
		So(err, ShouldBeNil)
		So(s, ShouldNotBeEmpty)

		So(tm.String(), ShouldResemble, "TxUnknown")

		tm = TxType(0)
		So(tm.String(), ShouldResemble, "TxBilling")
	})
}
