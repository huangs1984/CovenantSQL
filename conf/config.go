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

package conf

import (
	"io/ioutil"
	"time"

	"gitlab.com/thunderdb/ThunderDB/crypto/asymmetric"
	"gitlab.com/thunderdb/ThunderDB/pow/cpuminer"
	"gitlab.com/thunderdb/ThunderDB/proto"
	"gitlab.com/thunderdb/ThunderDB/utils/log"
	"gopkg.in/yaml.v2"
)

// ServerRole defines the role of node to be leader/coordinator in peer set.
type ServerRole int

// these const specify the role of this app, which can be "miner", "blockProducer"
const (
	MinerBuildTag         = "M"
	BlockProducerBuildTag = "B"
	ClientBuildTag        = "C"
	UnknownBuildTag       = "U"
)

// RoleTag indicate which role the daemon is playing
var RoleTag = UnknownBuildTag

const (
	// Leader is a server that have the ability to organize committing requests.
	Leader ServerRole = iota
	// Follower is a server that follow the leader log commits.
	Follower
	// Miner is a server that run sql database
	Miner
	// Client is a client that send sql query to database
	Client
)

func (s ServerRole) String() string {
	switch s {
	case Leader:
		return "Leader"
	case Follower:
		return "Follower"
	case Miner:
		return "Miner"
	case Client:
		return "Client"
	}
	return "Unknown"
}

// BPInfo hold all BP info fields
type BPInfo struct {
	// PublicKeyStr is the public key of Block Producer
	PublicKeyStr string `yaml:"PublicKeyStr"`
	// PublicKey point to BlockProducer public key
	PublicKey *asymmetric.PublicKey `yaml:"-"`
	// NodeID is the node id of Block Producer
	NodeID proto.NodeID `yaml:"NodeID"`
	// RawNodeID
	RawNodeID proto.RawNodeID `yaml:"-"`
	// Nonce is the nonce, SEE: cmd/idminer for more
	Nonce cpuminer.Uint256 `yaml:"Nonce"`
}

// NodeInfo for conf generation and load purpose.
type NodeInfo struct {
	ID        proto.NodeID
	Nonce     cpuminer.Uint256
	PublicKey *asymmetric.PublicKey `yaml:"-"`
	Addr      string
	Role      ServerRole
}

// MinerDatabaseFixture config.
type MinerDatabaseFixture struct {
	DatabaseID               proto.DatabaseID `yaml:"DatabaseID"`
	Term                     uint64           `yaml:"Term"`
	Leader                   proto.NodeID     `yaml:"Leader"`
	Servers                  []proto.NodeID   `yaml:"Servers"`
	GenesisBlockFile         string           `yaml:"GenesisBlockFile"`
	AutoGenerateGenesisBlock bool             `yaml:"AutoGenerateGenesisBlock,omitempty"`
}

// MinerConfig for miner config.
type MinerInfo struct {
	// node basic config
	RootDir               string        `yaml:"RootDir"`
	MaxReqTimeGap         time.Duration `yaml:"MaxReqTimeGap,omitempty"`
	MetricCollectInterval time.Duration `yaml:"MetricCollectInterval,omitempty"`

	// when test mode, fixture database config is used.
	IsTestMode   bool                    `yaml:"IsTestMode,omitempty"`
	TestFixtures []*MinerDatabaseFixture `yaml:"TestFixtures,omitempty"`
}

// Config holds all the config read from yaml config file
type Config struct {
	IsTestMode      bool // when testMode use default empty masterKey
	GenerateKeyPair bool `yaml:"-"`
	//TODO(auxten): set yaml key for config
	WorkingRoot     string
	PubKeyStoreFile string
	PrivateKeyFile  string
	DHTFileName     string
	ListenAddr      string
	ThisNodeID      proto.NodeID
	ValidDNSKeys    map[string]string `yaml:"ValidDNSKeys"` // map[DNSKEY]domain

	BP    *BPInfo    `yaml:"BlockProducer"`
	Miner *MinerInfo `yaml:"Miner,omitempty"`

	KnownNodes *[]NodeInfo
}

// GConf is the global config pointer
var GConf *Config

// LoadConfig loads config from configPath
func LoadConfig(configPath string) (config *Config, err error) {
	configBytes, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Errorf("read config file failed: %s", err)
		return
	}
	config = &Config{}
	err = yaml.Unmarshal(configBytes, config)
	if err != nil {
		log.Errorf("unmarshal config file failed: %s", err)
		return
	}
	return
}
