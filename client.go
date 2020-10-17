/*
 * Copyright 2019 Aletheia Ware LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package spaceclientgo

import (
	"bytes"
	"crypto/rsa"
	"encoding/base64"
	"github.com/AletheiaWareLLC/aliasgo"
	"github.com/AletheiaWareLLC/bcclientgo"
	"github.com/AletheiaWareLLC/bcgo"
	"github.com/AletheiaWareLLC/financego"
	"github.com/AletheiaWareLLC/spacego"
	"github.com/golang/protobuf/proto"
	"io"
	"log"
	"net/http"
)

type MetaCallback func(entry *bcgo.BlockEntry, meta *spacego.Meta) error

type SpaceClient struct {
	bcclientgo.BCClient
}

func NewSpaceClient(peers ...string) *SpaceClient {
	if len(peers) == 0 {
		peers = append(
			spacego.GetSpaceHosts(), // Add SPACE host as peer
			bcgo.GetBCHost(),        // Add BC host as peer
		)
	}
	return &SpaceClient{
		BCClient: *bcclientgo.NewBCClient(peers...),
	}
}

func (c *SpaceClient) Init(listener bcgo.MiningListener) (*bcgo.Node, error) {
	root, err := c.GetRoot()
	if err != nil {
		return nil, err
	}

	// Add Space hosts to peers
	for _, host := range spacego.GetSpaceHosts() {
		if err := bcgo.AddPeer(root, host); err != nil {
			return nil, err
		}
	}

	// Add BC host to peers
	if err := bcgo.AddPeer(root, bcgo.GetBCHost()); err != nil {
		return nil, err
	}

	return c.BCClient.Init(listener)
}

// Adds file
func (c *SpaceClient) Add(node *bcgo.Node, listener bcgo.MiningListener, name, mime string, reader io.Reader) (*bcgo.Reference, error) {
	// TODO compress data

	metas := spacego.OpenMetaChannel(node.Alias)
	if err := metas.Refresh(node.Cache, node.Network); err != nil {
		log.Println(err)
	}

	files := spacego.OpenFileChannel(node.Alias)
	if err := files.Refresh(node.Cache, node.Network); err != nil {
		log.Println(err)
	}

	acl := map[string]*rsa.PublicKey{
		node.Alias: &node.Key.PublicKey,
	}

	var references []*bcgo.Reference

	size, err := bcgo.CreateRecords(node.Alias, node.Key, acl, nil, reader, func(key []byte, record *bcgo.Record) error {
		reference, err := bcgo.WriteRecord(files.Name, node.Cache, record)
		if err != nil {
			return err
		}
		references = append(references, reference)
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Mine file channel
	if _, _, err := node.Mine(files, bcgo.THRESHOLD_I, listener); err != nil {
		return nil, err
	}

	// Push update to peers
	if err := files.Push(node.Cache, node.Network); err != nil {
		log.Println(err)
	}

	// TODO Add preview

	meta := spacego.Meta{
		Name: name,
		Size: uint64(size),
		Type: mime,
	}

	data, err := proto.Marshal(&meta)
	if err != nil {
		return nil, err
	}

	// Write meta data
	reference, err := node.Write(bcgo.Timestamp(), metas, acl, references, data)
	if err != nil {
		return nil, err
	}

	// Mine meta channel
	if _, _, err := node.Mine(metas, bcgo.THRESHOLD_G, listener); err != nil {
		return nil, err
	}

	// Push update to peers
	if err := metas.Push(node.Cache, node.Network); err != nil {
		log.Println(err)
	}
	return reference, nil
}

// Adds file using Remote Mining Service
func (c *SpaceClient) AddRemote(node *bcgo.Node, domain, name, mime string, reader io.Reader) (*bcgo.Reference, error) {
	// TODO compress data

	acl := map[string]*rsa.PublicKey{
		node.Alias: &node.Key.PublicKey,
	}

	var references []*bcgo.Reference

	size, err := bcgo.CreateRecords(node.Alias, node.Key, acl, nil, reader, func(key []byte, record *bcgo.Record) error {
		request, err := spacego.CreateRemoteMiningRequest("https://"+domain, "file", record)
		if err != nil {
			return err
		}
		client := http.Client{}
		response, err := client.Do(request)
		if err != nil {
			return err
		}
		reference, err := spacego.ParseRemoteMiningResponse(response)
		if err != nil {
			return err
		}
		references = append(references, reference)
		return nil
	})
	if err != nil {
		return nil, err
	}

	// TODO Add preview

	meta := spacego.Meta{
		Name: name,
		Size: uint64(size),
		Type: mime,
	}

	data, err := proto.Marshal(&meta)
	if err != nil {
		return nil, err
	}

	_, record, err := bcgo.CreateRecord(bcgo.Timestamp(), node.Alias, node.Key, acl, references, data)
	if err != nil {
		return nil, err
	}

	request, err := spacego.CreateRemoteMiningRequest("https://"+domain, "meta", record)
	if err != nil {
		return nil, err
	}
	client := http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	return spacego.ParseRemoteMiningResponse(response)
}

// List files owned by key
func (c *SpaceClient) List(node *bcgo.Node, callback MetaCallback) error {
	metas := spacego.OpenMetaChannel(node.Alias)
	if err := metas.Refresh(node.Cache, node.Network); err != nil {
		log.Println(err)
	}
	return spacego.GetMeta(metas, node.Cache, node.Network, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, meta *spacego.Meta) error {
		return callback(entry, meta)
	})
}

// List files shared with key
func (c *SpaceClient) ListShared(node *bcgo.Node, callback MetaCallback) error {
	shares := spacego.OpenShareChannel(node.Alias)
	if err := shares.Refresh(node.Cache, node.Network); err != nil {
		log.Println(err)
	}
	return spacego.GetShare(shares, node.Cache, node.Network, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, share *spacego.Share) error {
		if share.MetaReference == nil {
			// Meta reference not set
			return nil
		}
		metas := spacego.OpenMetaChannel(entry.Record.Creator)
		if err := metas.Refresh(node.Cache, node.Network); err != nil {
			log.Println(err)
		}
		return spacego.GetSharedMeta(metas, node.Cache, node.Network, share.MetaReference.RecordHash, share.MetaKey, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
			return callback(entry, meta)
		})
	})
}

// Get file owned by key with given hash
func (c *SpaceClient) Get(node *bcgo.Node, recordHash []byte, callback MetaCallback) error {
	metas := spacego.OpenMetaChannel(node.Alias)
	if err := metas.Refresh(node.Cache, node.Network); err != nil {
		log.Println(err)
	}
	return spacego.GetMeta(metas, node.Cache, node.Network, node.Alias, node.Key, recordHash, func(entry *bcgo.BlockEntry, key []byte, meta *spacego.Meta) error {
		return callback(entry, meta)
	})
}

// Get file shared to key with given hash
func (c *SpaceClient) GetShared(node *bcgo.Node, recordHash []byte, callback MetaCallback) error {
	shares := spacego.OpenShareChannel(node.Alias)
	if err := shares.Refresh(node.Cache, node.Network); err != nil {
		log.Println(err)
	}
	return spacego.GetShare(shares, node.Cache, node.Network, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, share *spacego.Share) error {
		if share.MetaReference != nil && bytes.Equal(recordHash, share.MetaReference.RecordHash) {
			metas := spacego.OpenMetaChannel(entry.Record.Creator)
			if err := metas.Refresh(node.Cache, node.Network); err != nil {
				log.Println(err)
			}
			return spacego.GetSharedMeta(metas, node.Cache, node.Network, share.MetaReference.RecordHash, share.MetaKey, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
				return callback(entry, meta)
			})
		}
		return nil
	})
}

// Get all files owned by key with given mime-types
func (c *SpaceClient) GetAll(node *bcgo.Node, mime []string, callback MetaCallback) error {
	metas := spacego.OpenMetaChannel(node.Alias)
	if err := metas.Refresh(node.Cache, node.Network); err != nil {
		log.Println(err)
	}
	return spacego.GetMeta(metas, node.Cache, node.Network, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, meta *spacego.Meta) error {
		for _, m := range mime {
			if meta.Type == m {
				return callback(entry, meta)
			}
		}
		return nil
	})
}

// Get all files shared to key with given mime-types
func (c *SpaceClient) GetAllShared(node *bcgo.Node, mime []string, callback MetaCallback) error {
	shares := spacego.OpenShareChannel(node.Alias)
	if err := shares.Refresh(node.Cache, node.Network); err != nil {
		log.Println(err)
	}
	return spacego.GetShare(shares, node.Cache, node.Network, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, share *spacego.Share) error {
		if share.MetaReference == nil {
			// Meta reference not set
			return nil
		}
		metas := spacego.OpenMetaChannel(entry.Record.Creator)
		if err := metas.Refresh(node.Cache, node.Network); err != nil {
			log.Println(err)
		}
		return spacego.GetSharedMeta(metas, node.Cache, node.Network, share.MetaReference.RecordHash, share.MetaKey, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
			for _, m := range mime {
				if meta.Type == m {
					return callback(entry, meta)
				}
			}
			return nil
		})
	})
}

// Read file by given hash
func (c *SpaceClient) Read(node *bcgo.Node, recordHash []byte, writer io.Writer) (uint64, error) {
	// TODO read from cache if file exists
	count := uint64(0)
	files := spacego.OpenFileChannel(node.Alias)
	if err := files.Refresh(node.Cache, node.Network); err != nil {
		log.Println(err)
	}
	metas := spacego.OpenMetaChannel(node.Alias)
	if err := metas.Refresh(node.Cache, node.Network); err != nil {
		log.Println(err)
	}
	if err := spacego.GetMeta(metas, node.Cache, node.Network, node.Alias, node.Key, recordHash, func(entry *bcgo.BlockEntry, key []byte, meta *spacego.Meta) error {
		for _, reference := range entry.Record.Reference {
			// TODO this is inefficient
			if err := spacego.GetFile(files, node.Cache, node.Network, node.Alias, node.Key, reference.RecordHash, func(entry *bcgo.BlockEntry, key, data []byte) error {
				n, err := writer.Write(data)
				if err != nil {
					return err
				}
				count += uint64(n)
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return 0, err
	}
	return count, nil
}

// Read file shared to key with given hash
func (c *SpaceClient) ReadShared(node *bcgo.Node, recordHash []byte, writer io.Writer) (uint64, error) {
	// TODO read from cache if file exists
	count := uint64(0)
	shares := spacego.OpenShareChannel(node.Alias)
	if err := shares.Refresh(node.Cache, node.Network); err != nil {
		log.Println(err)
	}
	if err := spacego.GetShare(shares, node.Cache, node.Network, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, share *spacego.Share) error {
		if share.MetaReference != nil && bytes.Equal(recordHash, share.MetaReference.RecordHash) {
			metas := spacego.OpenMetaChannel(entry.Record.Creator)
			if err := metas.Refresh(node.Cache, node.Network); err != nil {
				log.Println(err)
			}
			files := spacego.OpenFileChannel(entry.Record.Creator)
			if err := files.Refresh(node.Cache, node.Network); err != nil {
				log.Println(err)
			}
			if err := spacego.GetSharedMeta(metas, node.Cache, node.Network, share.MetaReference.RecordHash, share.MetaKey, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
				for index, reference := range entry.Record.Reference {
					if err := spacego.GetSharedFile(files, node.Cache, node.Network, reference.RecordHash, share.ChunkKey[index], func(entry *bcgo.BlockEntry, data []byte) error {
						n, err := writer.Write(data)
						if err != nil {
							return err
						}
						count += uint64(n)
						return nil
					}); err != nil {
						return err
					}
				}
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return 0, err
	}
	return count, nil
}

// Share file with recipients
func (c *SpaceClient) Share(node *bcgo.Node, listener bcgo.MiningListener, recordHash []byte, recipients []string) error {
	aliases := aliasgo.OpenAliasChannel()
	if err := aliases.Refresh(node.Cache, node.Network); err != nil {
		log.Println(err)
	}
	metas := spacego.OpenMetaChannel(node.Alias)
	if err := metas.Refresh(node.Cache, node.Network); err != nil {
		log.Println(err)
	}
	files := spacego.OpenFileChannel(node.Alias)
	if err := files.Refresh(node.Cache, node.Network); err != nil {
		log.Println(err)
	}
	return spacego.GetMeta(metas, node.Cache, node.Network, node.Alias, node.Key, recordHash, func(entry *bcgo.BlockEntry, key []byte, meta *spacego.Meta) error {
		chunkKeys := make([][]byte, len(entry.Record.Reference))
		for index, reference := range entry.Record.Reference {
			if err := bcgo.ReadKey(files.Name, files.Head, nil, node.Cache, node.Network, node.Alias, node.Key, reference.RecordHash, func(key []byte) error {
				chunkKeys[index] = key
				return nil
			}); err != nil {
				return err
			}
		}
		share := spacego.Share{
			MetaReference: &bcgo.Reference{
				Timestamp:   entry.Record.Timestamp,
				ChannelName: metas.Name,
				RecordHash:  recordHash,
			},
			MetaKey:  key,
			ChunkKey: chunkKeys,
			// TODO PreviewReference:
			// TODO PreviewKey:
		}
		data, err := proto.Marshal(&share)
		if err != nil {
			return err
		}

		for _, alias := range recipients {
			shares := spacego.OpenShareChannel(alias)
			if err := shares.Refresh(node.Cache, node.Network); err != nil {
				log.Println(err)
			}

			publicKey, err := aliasgo.GetPublicKey(aliases, node.Cache, node.Network, alias)
			if err != nil {
				return err
			}
			acl := map[string]*rsa.PublicKey{
				alias:      publicKey,
				node.Alias: &node.Key.PublicKey,
			}
			if _, err := node.Write(bcgo.Timestamp(), shares, acl, nil, data); err != nil {
				return err
			}
			if _, _, err := node.Mine(shares, bcgo.THRESHOLD_G, listener); err != nil {
				return err
			}
		}
		return nil
	})
}

// Search files owned by key
func (c *SpaceClient) Search(node *bcgo.Node, terms []string, callback MetaCallback) error {
	metas := spacego.OpenMetaChannel(node.Alias)
	if err := metas.Refresh(node.Cache, node.Network); err != nil {
		log.Println(err)
	}
	if err := spacego.GetMeta(metas, node.Cache, node.Network, node.Alias, node.Key, nil, func(metaEntry *bcgo.BlockEntry, metaKey []byte, meta *spacego.Meta) error {
		tags := spacego.OpenTagChannel(base64.RawURLEncoding.EncodeToString(metaEntry.RecordHash))
		if err := tags.Refresh(node.Cache, node.Network); err != nil {
			log.Println(err)
		}
		return spacego.GetTag(tags, node.Cache, node.Network, node.Alias, node.Key, nil, func(tagEntry *bcgo.BlockEntry, tagKey []byte, tag *spacego.Tag) error {
			for _, value := range terms {
				if tag.Value == value {
					return callback(metaEntry, meta)
				}
			}
			return nil
		})
	}); err != nil {
		return err
	}
	return nil
}

// Search files shared with key
func (c *SpaceClient) SearchShared(node *bcgo.Node, terms []string, callback MetaCallback) error {
	shares := spacego.OpenShareChannel(node.Alias)
	if err := shares.Refresh(node.Cache, node.Network); err != nil {
		log.Println(err)
	}
	if err := spacego.GetShare(shares, node.Cache, node.Network, node.Alias, node.Key, nil, func(shareEntry *bcgo.BlockEntry, shareKey []byte, share *spacego.Share) error {
		if share.MetaReference == nil {
			// Meta reference not set
			return nil
		}
		metas := spacego.OpenMetaChannel(shareEntry.Record.Creator)
		if err := metas.Refresh(node.Cache, node.Network); err != nil {
			log.Println(err)
		}
		if err := spacego.GetSharedMeta(metas, node.Cache, node.Network, share.MetaReference.RecordHash, share.MetaKey, func(metaEntry *bcgo.BlockEntry, meta *spacego.Meta) error {
			tags := spacego.OpenTagChannel(base64.RawURLEncoding.EncodeToString(metaEntry.RecordHash))
			if err := tags.Refresh(node.Cache, node.Network); err != nil {
				log.Println(err)
			}
			return spacego.GetTag(tags, node.Cache, node.Network, node.Alias, node.Key, nil, func(tagEntry *bcgo.BlockEntry, tagKey []byte, tag *spacego.Tag) error {
				for _, value := range terms {
					if tag.Value == value {
						return callback(metaEntry, meta)
					}
				}
				return nil
			})
		}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// Tag file owned by key
func (c *SpaceClient) Tag(node *bcgo.Node, listener bcgo.MiningListener, recordHash []byte, tag []string) ([]*bcgo.Reference, error) {
	metas := spacego.OpenMetaChannel(node.Alias)
	if err := metas.Refresh(node.Cache, node.Network); err != nil {
		log.Println(err)
	}
	tags := spacego.OpenTagChannel(base64.RawURLEncoding.EncodeToString(recordHash))
	if err := tags.Refresh(node.Cache, node.Network); err != nil {
		log.Println(err)
	}
	var references []*bcgo.Reference
	if err := spacego.GetMeta(metas, node.Cache, node.Network, node.Alias, node.Key, recordHash, func(entry *bcgo.BlockEntry, key []byte, meta *spacego.Meta) error {
		for _, t := range tag {
			tag := spacego.Tag{
				Value: t,
			}
			data, err := proto.Marshal(&tag)
			if err != nil {
				return err
			}
			acl := map[string]*rsa.PublicKey{
				node.Alias: &node.Key.PublicKey,
			}
			references := []*bcgo.Reference{&bcgo.Reference{
				Timestamp:   entry.Record.Timestamp,
				ChannelName: metas.Name,
				RecordHash:  recordHash,
			}}
			reference, err := node.Write(bcgo.Timestamp(), tags, acl, references, data)
			if err != nil {
				return err
			}
			references = append(references, reference)
			if _, _, err := node.Mine(tags, bcgo.THRESHOLD_G, listener); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return references, nil
}

// Tag file shared with key
func (c *SpaceClient) TagShared(node *bcgo.Node, listener bcgo.MiningListener, recordHash []byte, tag []string) ([]*bcgo.Reference, error) {
	metas := spacego.OpenMetaChannel(node.Alias)
	if err := metas.Refresh(node.Cache, node.Network); err != nil {
		log.Println(err)
	}
	shares := spacego.OpenShareChannel(node.Alias)
	if err := shares.Refresh(node.Cache, node.Network); err != nil {
		log.Println(err)
	}
	tags := spacego.OpenTagChannel(base64.RawURLEncoding.EncodeToString(recordHash))
	if err := tags.Refresh(node.Cache, node.Network); err != nil {
		log.Println(err)
	}
	var references []*bcgo.Reference
	if err := spacego.GetShare(shares, node.Cache, node.Network, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, share *spacego.Share) error {
		if share.MetaReference != nil && bytes.Equal(recordHash, share.MetaReference.RecordHash) {
			sharedMetas := spacego.OpenMetaChannel(entry.Record.Creator)
			if err := sharedMetas.Refresh(node.Cache, node.Network); err != nil {
				log.Println(err)
			}
			if err := spacego.GetSharedMeta(sharedMetas, node.Cache, node.Network, recordHash, share.MetaKey, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
				for _, t := range tag {
					tag := spacego.Tag{
						Value: t,
					}
					data, err := proto.Marshal(&tag)
					if err != nil {
						return err
					}
					acl := map[string]*rsa.PublicKey{
						node.Alias: &node.Key.PublicKey,
					}
					reference, err := node.Write(bcgo.Timestamp(), tags, acl, []*bcgo.Reference{
						&bcgo.Reference{
							Timestamp:   entry.Record.Timestamp,
							ChannelName: metas.Name,
							RecordHash:  recordHash,
						},
					}, data)
					if err != nil {
						return err
					}
					references = append(references, reference)
					if _, _, err := node.Mine(tags, bcgo.THRESHOLD_G, listener); err != nil {
						return err
					}
				}
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return references, nil
}

// Get all tags for the file with the given hash
func (c *SpaceClient) GetTag(node *bcgo.Node, recordHash []byte, callback func(entry *bcgo.BlockEntry, tag *spacego.Tag)) error {
	tags := spacego.OpenTagChannel(base64.RawURLEncoding.EncodeToString(recordHash))
	if err := tags.Refresh(node.Cache, node.Network); err != nil {
		log.Println(err)
	}
	return spacego.GetTag(tags, node.Cache, node.Network, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, tag *spacego.Tag) error {
		for _, reference := range entry.Record.Reference {
			if bytes.Equal(recordHash, reference.RecordHash) {
				callback(entry, tag)
			}
		}
		return nil
	})
}

func (c *SpaceClient) GetRegistration(merchant string, callback func(*financego.Registration) error) error {
	node, err := c.GetNode()
	if err != nil {
		return err
	}
	registrations := spacego.OpenRegistrationChannel()
	if err := registrations.Refresh(node.Cache, node.Network); err != nil {
		log.Println(err)
	}
	return financego.GetRegistrationAsync(registrations, node.Cache, node.Network, merchant, nil, node.Alias, node.Key, callback)
}

func (c *SpaceClient) GetSubscription(merchant string, callback func(*financego.Subscription) error) error {
	node, err := c.GetNode()
	if err != nil {
		return err
	}
	subscriptions := spacego.OpenSubscriptionChannel()
	if err := subscriptions.Refresh(node.Cache, node.Network); err != nil {
		log.Println(err)
	}
	return financego.GetSubscriptionAsync(subscriptions, node.Cache, node.Network, merchant, nil, node.Alias, node.Key, "", "", callback)
}
