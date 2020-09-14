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

type Client struct {
	bcclientgo.BCClient
}

func (c *Client) Init(listener bcgo.MiningListener) (*bcgo.Node, error) {
	// Add Space hosts to peers
	for _, host := range spacego.GetSpaceHosts() {
		if err := bcgo.AddPeer(c.Root, host); err != nil {
			return nil, err
		}
	}

	// Add BC host to peers
	if err := bcgo.AddPeer(c.Root, bcgo.GetBCHost()); err != nil {
		return nil, err
	}

	return c.BCClient.Init(listener)
}

// Adds file
func (c *Client) Add(node *bcgo.Node, listener bcgo.MiningListener, name, mime string, reader io.Reader) (*bcgo.Reference, error) {
	// TODO compress data

	metas := spacego.OpenMetaChannel(node.Alias)
	if err := metas.Refresh(c.Cache, c.Network); err != nil {
		log.Println(err)
	}

	files := spacego.OpenFileChannel(node.Alias)
	if err := files.LoadHead(c.Cache, c.Network); err != nil {
		log.Println(err)
	}

	acl := map[string]*rsa.PublicKey{
		node.Alias: &node.Key.PublicKey,
	}

	var references []*bcgo.Reference

	size, err := bcgo.CreateRecords(node.Alias, node.Key, acl, nil, reader, func(key []byte, record *bcgo.Record) error {
		reference, err := bcgo.WriteRecord(files.Name, c.Cache, record)
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
func (c *Client) AddRemote(node *bcgo.Node, domain, name, mime string, reader io.Reader) (*bcgo.Reference, error) {
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
func (c *Client) List(node *bcgo.Node, callback MetaCallback) error {
	metas := spacego.OpenMetaChannel(node.Alias)
	if err := metas.Refresh(c.Cache, c.Network); err != nil {
		log.Println(err)
	}
	return spacego.GetMeta(metas, c.Cache, c.Network, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, meta *spacego.Meta) error {
		return callback(entry, meta)
	})
}

// List files shared with key
func (c *Client) ListShared(node *bcgo.Node, callback MetaCallback) error {
	shares := spacego.OpenShareChannel(node.Alias)
	if err := shares.Refresh(c.Cache, c.Network); err != nil {
		log.Println(err)
	}
	return spacego.GetShare(shares, c.Cache, c.Network, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, share *spacego.Share) error {
		if share.MetaReference == nil {
			// Meta reference not set
			return nil
		}
		metas := spacego.OpenMetaChannel(entry.Record.Creator)
		if err := metas.Refresh(c.Cache, c.Network); err != nil {
			log.Println(err)
		}
		return spacego.GetSharedMeta(metas, c.Cache, c.Network, share.MetaReference.RecordHash, share.MetaKey, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
			return callback(entry, meta)
		})
	})
}

// Show file owned by key with given hash
func (c *Client) Show(node *bcgo.Node, recordHash []byte, callback MetaCallback) error {
	metas := spacego.OpenMetaChannel(node.Alias)
	if err := metas.Refresh(c.Cache, c.Network); err != nil {
		log.Println(err)
	}
	return spacego.GetMeta(metas, c.Cache, c.Network, node.Alias, node.Key, recordHash, func(entry *bcgo.BlockEntry, key []byte, meta *spacego.Meta) error {
		return callback(entry, meta)
	})
}

// Show file shared to key with given hash
func (c *Client) ShowShared(node *bcgo.Node, recordHash []byte, callback MetaCallback) error {
	shares := spacego.OpenShareChannel(node.Alias)
	if err := shares.Refresh(c.Cache, c.Network); err != nil {
		log.Println(err)
	}
	return spacego.GetShare(shares, c.Cache, c.Network, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, share *spacego.Share) error {
		if share.MetaReference != nil && bytes.Equal(recordHash, share.MetaReference.RecordHash) {
			metas := spacego.OpenMetaChannel(entry.Record.Creator)
			if err := metas.Refresh(c.Cache, c.Network); err != nil {
				log.Println(err)
			}
			return spacego.GetSharedMeta(metas, c.Cache, c.Network, share.MetaReference.RecordHash, share.MetaKey, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
				return callback(entry, meta)
			})
		}
		return nil
	})
}

// Show all files owned by key with given mime-type
func (c *Client) ShowAll(node *bcgo.Node, mime string, callback MetaCallback) error {
	metas := spacego.OpenMetaChannel(node.Alias)
	if err := metas.Refresh(c.Cache, c.Network); err != nil {
		log.Println(err)
	}
	return spacego.GetMeta(metas, c.Cache, c.Network, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, meta *spacego.Meta) error {
		if meta.Type == mime {
			return callback(entry, meta)
		}
		return nil
	})
}

// Show all files shared to key with given mime-type
func (c *Client) ShowAllShared(node *bcgo.Node, mime string, callback MetaCallback) error {
	shares := spacego.OpenShareChannel(node.Alias)
	if err := shares.Refresh(c.Cache, c.Network); err != nil {
		log.Println(err)
	}
	return spacego.GetShare(shares, c.Cache, c.Network, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, share *spacego.Share) error {
		if share.MetaReference == nil {
			// Meta reference not set
			return nil
		}
		metas := spacego.OpenMetaChannel(entry.Record.Creator)
		if err := metas.Refresh(c.Cache, c.Network); err != nil {
			log.Println(err)
		}
		return spacego.GetSharedMeta(metas, c.Cache, c.Network, share.MetaReference.RecordHash, share.MetaKey, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
			if meta.Type == mime {
				return callback(entry, meta)
			}
			return nil
		})
	})
}

// Get file by given hash
func (c *Client) Get(node *bcgo.Node, recordHash []byte, writer io.Writer) (uint64, error) {
	count := uint64(0)
	files := spacego.OpenFileChannel(node.Alias)
	if err := files.LoadHead(c.Cache, c.Network); err != nil {
		log.Println(err)
	}
	metas := spacego.OpenMetaChannel(node.Alias)
	if err := metas.Refresh(c.Cache, c.Network); err != nil {
		log.Println(err)
	}
	if err := spacego.GetMeta(metas, c.Cache, c.Network, node.Alias, node.Key, recordHash, func(entry *bcgo.BlockEntry, key []byte, meta *spacego.Meta) error {
		for _, reference := range entry.Record.Reference {
			// TODO this is inefficient
			if err := spacego.GetFile(files, c.Cache, c.Network, node.Alias, node.Key, reference.RecordHash, func(entry *bcgo.BlockEntry, key, data []byte) error {
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

// Get file shared to key with given hash
func (c *Client) GetShared(node *bcgo.Node, recordHash []byte, writer io.Writer) (uint64, error) {
	count := uint64(0)
	shares := spacego.OpenShareChannel(node.Alias)
	if err := shares.Refresh(c.Cache, c.Network); err != nil {
		log.Println(err)
	}
	if err := spacego.GetShare(shares, c.Cache, c.Network, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, share *spacego.Share) error {
		if share.MetaReference != nil && bytes.Equal(recordHash, share.MetaReference.RecordHash) {
			metas := spacego.OpenMetaChannel(entry.Record.Creator)
			if err := metas.Refresh(c.Cache, c.Network); err != nil {
				log.Println(err)
			}
			files := spacego.OpenFileChannel(entry.Record.Creator)
			if err := files.LoadHead(c.Cache, c.Network); err != nil {
				log.Println(err)
			}
			if err := spacego.GetSharedMeta(metas, c.Cache, c.Network, share.MetaReference.RecordHash, share.MetaKey, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
				for index, reference := range entry.Record.Reference {
					if err := spacego.GetSharedFile(files, c.Cache, c.Network, reference.RecordHash, share.ChunkKey[index], func(entry *bcgo.BlockEntry, data []byte) error {
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

func (c *Client) Share(node *bcgo.Node, listener bcgo.MiningListener, recordHash []byte, recipients []string) error {
	aliases := aliasgo.OpenAliasChannel()
	if err := aliases.Refresh(c.Cache, c.Network); err != nil {
		log.Println(err)
	}
	metas := spacego.OpenMetaChannel(node.Alias)
	if err := metas.Refresh(c.Cache, c.Network); err != nil {
		log.Println(err)
	}
	files := spacego.OpenFileChannel(node.Alias)
	if err := files.LoadHead(c.Cache, c.Network); err != nil {
		log.Println(err)
	}
	return spacego.GetMeta(metas, c.Cache, c.Network, node.Alias, node.Key, recordHash, func(entry *bcgo.BlockEntry, key []byte, meta *spacego.Meta) error {
		chunkKeys := make([][]byte, len(entry.Record.Reference))
		for index, reference := range entry.Record.Reference {
			if err := bcgo.ReadKey(files.Name, files.Head, nil, c.Cache, c.Network, node.Alias, node.Key, reference.RecordHash, func(key []byte) error {
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
			if err := shares.Refresh(c.Cache, c.Network); err != nil {
				log.Println(err)
			}

			publicKey, err := aliasgo.GetPublicKey(aliases, c.Cache, c.Network, alias)
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
func (c *Client) Search(node *bcgo.Node, terms []string, callback MetaCallback) error {
	metas := spacego.OpenMetaChannel(node.Alias)
	if err := metas.Refresh(c.Cache, c.Network); err != nil {
		log.Println(err)
	}
	if err := spacego.GetMeta(metas, c.Cache, c.Network, node.Alias, node.Key, nil, func(metaEntry *bcgo.BlockEntry, metaKey []byte, meta *spacego.Meta) error {
		tags := spacego.OpenTagChannel(base64.RawURLEncoding.EncodeToString(metaEntry.RecordHash))
		if err := tags.Refresh(c.Cache, c.Network); err != nil {
			log.Println(err)
		}
		return spacego.GetTag(tags, c.Cache, c.Network, node.Alias, node.Key, nil, func(tagEntry *bcgo.BlockEntry, tagKey []byte, tag *spacego.Tag) error {
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
func (c *Client) SearchShared(node *bcgo.Node, terms []string, callback MetaCallback) error {
	shares := spacego.OpenShareChannel(node.Alias)
	if err := shares.Refresh(c.Cache, c.Network); err != nil {
		log.Println(err)
	}
	if err := spacego.GetShare(shares, c.Cache, c.Network, node.Alias, node.Key, nil, func(shareEntry *bcgo.BlockEntry, shareKey []byte, share *spacego.Share) error {
		if share.MetaReference == nil {
			// Meta reference not set
			return nil
		}
		metas := spacego.OpenMetaChannel(shareEntry.Record.Creator)
		if err := metas.Refresh(c.Cache, c.Network); err != nil {
			log.Println(err)
		}
		if err := spacego.GetSharedMeta(metas, c.Cache, c.Network, share.MetaReference.RecordHash, share.MetaKey, func(metaEntry *bcgo.BlockEntry, meta *spacego.Meta) error {
			tags := spacego.OpenTagChannel(base64.RawURLEncoding.EncodeToString(metaEntry.RecordHash))
			if err := tags.Refresh(c.Cache, c.Network); err != nil {
				log.Println(err)
			}
			return spacego.GetTag(tags, c.Cache, c.Network, node.Alias, node.Key, nil, func(tagEntry *bcgo.BlockEntry, tagKey []byte, tag *spacego.Tag) error {
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
func (c *Client) Tag(node *bcgo.Node, listener bcgo.MiningListener, recordHash []byte, tag []string) ([]*bcgo.Reference, error) {
	metas := spacego.OpenMetaChannel(node.Alias)
	if err := metas.Refresh(c.Cache, c.Network); err != nil {
		log.Println(err)
	}
	tags := spacego.OpenTagChannel(base64.RawURLEncoding.EncodeToString(recordHash))
	if err := tags.Refresh(c.Cache, c.Network); err != nil {
		log.Println(err)
	}
	var references []*bcgo.Reference
	if err := spacego.GetMeta(metas, c.Cache, c.Network, node.Alias, node.Key, recordHash, func(entry *bcgo.BlockEntry, key []byte, meta *spacego.Meta) error {
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
func (c *Client) TagShared(node *bcgo.Node, listener bcgo.MiningListener, recordHash []byte, tag []string) ([]*bcgo.Reference, error) {
	metas := spacego.OpenMetaChannel(node.Alias)
	if err := metas.Refresh(c.Cache, c.Network); err != nil {
		log.Println(err)
	}
	shares := spacego.OpenShareChannel(node.Alias)
	if err := shares.Refresh(c.Cache, c.Network); err != nil {
		log.Println(err)
	}
	tags := spacego.OpenTagChannel(base64.RawURLEncoding.EncodeToString(recordHash))
	if err := tags.Refresh(c.Cache, c.Network); err != nil {
		log.Println(err)
	}
	var references []*bcgo.Reference
	if err := spacego.GetShare(shares, c.Cache, c.Network, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, share *spacego.Share) error {
		if share.MetaReference != nil && bytes.Equal(recordHash, share.MetaReference.RecordHash) {
			sharedMetas := spacego.OpenMetaChannel(entry.Record.Creator)
			if err := sharedMetas.Refresh(c.Cache, c.Network); err != nil {
				log.Println(err)
			}
			if err := spacego.GetSharedMeta(sharedMetas, c.Cache, c.Network, recordHash, share.MetaKey, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
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

func (c *Client) ShowTag(node *bcgo.Node, recordHash []byte, callback func(entry *bcgo.BlockEntry, tag *spacego.Tag)) error {
	tags := spacego.OpenTagChannel(base64.RawURLEncoding.EncodeToString(recordHash))
	if err := tags.Refresh(c.Cache, c.Network); err != nil {
		log.Println(err)
	}
	return spacego.GetTag(tags, c.Cache, c.Network, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, tag *spacego.Tag) error {
		for _, reference := range entry.Record.Reference {
			if bytes.Equal(recordHash, reference.RecordHash) {
				callback(entry, tag)
			}
		}
		return nil
	})
}

func (c *Client) Registration(merchant string, callback func(*financego.Registration) error) error {
	node, err := bcgo.GetNode(c.Root, c.Cache, c.Network)
	if err != nil {
		return err
	}
	registrations := spacego.OpenRegistrationChannel()
	if err := registrations.Refresh(c.Cache, c.Network); err != nil {
		log.Println(err)
	}
	return financego.GetRegistrationAsync(registrations, c.Cache, c.Network, merchant, nil, node.Alias, node.Key, callback)
}

func (c *Client) Subscription(merchant string, callback func(*financego.Subscription) error) error {
	node, err := bcgo.GetNode(c.Root, c.Cache, c.Network)
	if err != nil {
		return err
	}
	subscriptions := spacego.OpenSubscriptionChannel()
	if err := subscriptions.Refresh(c.Cache, c.Network); err != nil {
		log.Println(err)
	}
	return financego.GetSubscriptionAsync(subscriptions, c.Cache, c.Network, merchant, nil, node.Alias, node.Key, "", "", callback)
}
