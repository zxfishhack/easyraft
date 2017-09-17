// Copyright 2015 zxfishhack
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Copyright 2015 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"path/filepath"
	"time"
)

type peer struct {
	ID  uint64 `json:"id"`
	URL string `json:"url"`
}

type config struct {
	ID               uint64 `json:"id"`
	ClusterID        uint64 `json:"cluster_id"`
	SnapCount        uint64 `json:"snap_count"`
	Waldir           string `json:"waldir"`
	Snapdir          string `json:"snapdir"`
	TickMs           uint64 `json:"tickms"`
	ElectionTick     int    `json:"election_tick"`
	HeartbeatTick    int    `json:"heartbeat_tick"`
	BootstrapTimeout int    `json:"boostrap_timeout"`
	Peers            []peer `json:"peers"`
	JoinExist        bool   `json:"join"`
	MaxSizePerMsg    uint64 `json:"max_size_per_msg"`
	MaxInflightMsgs  int    `json:"max_inflight_msgs"`
	SnapshotEntries  uint64 `json:"snapshot_entries"`
	MaxSnapFiles     uint   `json:"max_snap_files"`
	MaxWALFiles      uint   `json:"max_wal_files"`
}

func (c *config) backendPath() string { return filepath.Join(c.Snapdir, "db") }

//TODO complete this
func (c *config) VerifyBootstrap() error {
	return nil
}

func (c *config) peerDialTimeout() time.Duration {
	// 1s for queue wait and system delay
	// + one RTT, which is smaller than 1/5 election timeout
	return time.Second + time.Duration(c.ElectionTick)*time.Duration(c.TickMs)*time.Millisecond/5
}

func (c *config) bootstrapTimeout() time.Duration {
	return time.Duration(c.BootstrapTimeout) * time.Second
}

func (c *config) getPeerByID(id uint64) *peer {
	for _, peer := range c.Peers {
		if peer.ID == id {
			return &peer
		}
	}
	return nil
}
