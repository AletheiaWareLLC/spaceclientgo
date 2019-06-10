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

package main

import (
	"bytes"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"github.com/AletheiaWareLLC/aliasgo"
	"github.com/AletheiaWareLLC/bcgo"
	"github.com/AletheiaWareLLC/spacego"
	"github.com/golang/protobuf/proto"
	"io"
	"log"
	"net/http"
	"os"
)

type MetaCallback func(entry *bcgo.BlockEntry, meta *spacego.Meta) error

type Client struct {
	Root    string
	Cache   bcgo.Cache
	Network bcgo.Network
}

func (c *Client) Init() (*bcgo.Node, error) {
	// Add Space host to peers
	if err := bcgo.AddPeer(c.Root, spacego.GetSpaceHost()); err != nil {
		return nil, err
	}

	// Add BC host to peers
	if err := bcgo.AddPeer(c.Root, bcgo.GetBCHost()); err != nil {
		return nil, err
	}

	node, err := bcgo.GetNode(c.Root, c.Cache, c.Network)
	if err != nil {
		return nil, err
	}

	// Open Alias Channel
	aliases := aliasgo.OpenAndLoadAliasChannel(c.Cache, c.Network)
	if err := aliases.UniqueAlias(c.Cache, node.Alias); err != nil {
		return nil, err
	}
	if err := aliasgo.RegisterAlias(bcgo.GetBCWebsite(), node.Alias, node.Key); err != nil {
		// TODO if alias can't be registered with server, mine locally
		log.Println("Could not register alias: ", err)
		return nil, err
	}
	return node, nil
}

// Adds file
func (c *Client) Add(node *bcgo.Node, name, mime string, reader io.Reader) (*bcgo.Reference, error) {
	// TODO compress data

	files := spacego.OpenAndLoadFileChannel(node.Alias, c.Cache, c.Network)

	metas := spacego.OpenAndLoadMetaChannel(node.Alias, c.Cache, c.Network)

	acl := map[string]*rsa.PublicKey{
		node.Alias: &node.Key.PublicKey,
	}

	var references []*bcgo.Reference

	size, err := bcgo.CreateRecords(node.Alias, node.Key, acl, references, reader, func(key []byte, record *bcgo.Record) error {
		reference, err := bcgo.WriteRecord(files.GetName(), c.Cache, record)
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

	return node.Write(metas, acl, references, data)
}

// Adds file using Remote Mining Service
func (c *Client) AddRemote(node *bcgo.Node, name, mime string, reader io.Reader) (*bcgo.Reference, error) {
	// TODO compress data

	acl := map[string]*rsa.PublicKey{
		node.Alias: &node.Key.PublicKey,
	}

	var references []*bcgo.Reference

	size, err := bcgo.CreateRecords(node.Alias, node.Key, acl, references, reader, func(key []byte, record *bcgo.Record) error {
		request, err := spacego.CreateRemoteMiningRequest(spacego.GetSpaceWebsite(), "file", record)
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

	_, record, err := bcgo.CreateRecord(node.Alias, node.Key, acl, references, data)
	if err != nil {
		return nil, err
	}

	request, err := spacego.CreateRemoteMiningRequest(spacego.GetSpaceWebsite(), "meta", record)
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
	metas := spacego.OpenAndLoadMetaChannel(node.Alias, c.Cache, c.Network)
	return spacego.GetMeta(metas, c.Cache, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, meta *spacego.Meta) error {
		return callback(entry, meta)
	})
}

// List files shared with key
func (c *Client) ListShared(node *bcgo.Node, callback MetaCallback) error {
	shares := spacego.OpenAndLoadShareChannel(node.Alias, c.Cache, c.Network)
	return spacego.GetShare(shares, c.Cache, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, share *spacego.Share) error {
		if share.MetaReference == nil {
			// Meta reference not set
			return nil
		}
		return spacego.GetSharedMeta(c.Cache, c.Network, entry.Record.Creator, share.MetaReference.RecordHash, share.MetaKey, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
			return callback(entry, meta)
		})
	})
}

// Show file owned by key with given hash
func (c *Client) Show(node *bcgo.Node, recordHash []byte, callback MetaCallback) error {
	metas := spacego.OpenAndLoadMetaChannel(node.Alias, c.Cache, c.Network)
	return spacego.GetMeta(metas, c.Cache, node.Alias, node.Key, recordHash, func(entry *bcgo.BlockEntry, key []byte, meta *spacego.Meta) error {
		return callback(entry, meta)
	})
}

// Show file shared to key with given hash
func (c *Client) ShowShared(node *bcgo.Node, recordHash []byte, callback MetaCallback) error {
	shares := spacego.OpenAndLoadShareChannel(node.Alias, c.Cache, c.Network)
	return spacego.GetShare(shares, c.Cache, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, share *spacego.Share) error {
		if share.MetaReference != nil && bytes.Equal(recordHash, share.MetaReference.RecordHash) {
			return spacego.GetSharedMeta(c.Cache, c.Network, entry.Record.Creator, share.MetaReference.RecordHash, share.MetaKey, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
				return callback(entry, meta)
			})
		}
		return nil
	})
}

// Show all files owned by key with given mime-type
func (c *Client) ShowAll(node *bcgo.Node, mime string, callback MetaCallback) error {
	metas := spacego.OpenAndLoadMetaChannel(node.Alias, c.Cache, c.Network)
	return spacego.GetMeta(metas, c.Cache, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, meta *spacego.Meta) error {
		if meta.Type == mime {
			return callback(entry, meta)
		}
		return nil
	})
}

// Show all files shared to key with given mime-type
func (c *Client) ShowAllShared(node *bcgo.Node, mime string, callback MetaCallback) error {
	shares := spacego.OpenAndLoadShareChannel(node.Alias, c.Cache, c.Network)
	return spacego.GetShare(shares, c.Cache, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, share *spacego.Share) error {
		if share.MetaReference == nil {
			// Meta reference not set
			return nil
		}
		return spacego.GetSharedMeta(c.Cache, c.Network, entry.Record.Creator, share.MetaReference.RecordHash, share.MetaKey, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
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
	files := spacego.OpenAndLoadFileChannel(node.Alias, c.Cache, c.Network)
	metas := spacego.OpenAndLoadMetaChannel(node.Alias, c.Cache, c.Network)
	if err := spacego.GetMeta(metas, c.Cache, node.Alias, node.Key, recordHash, func(entry *bcgo.BlockEntry, key []byte, meta *spacego.Meta) error {
		for _, reference := range entry.Record.Reference {
			if err := spacego.GetFile(files, c.Cache, node.Alias, node.Key, reference.RecordHash, func(entry *bcgo.BlockEntry, key, data []byte) error {
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
	shares := spacego.OpenAndLoadShareChannel(node.Alias, c.Cache, c.Network)
	if err := spacego.GetShare(shares, c.Cache, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, share *spacego.Share) error {
		if share.MetaReference != nil && bytes.Equal(recordHash, share.MetaReference.RecordHash) {
			if err := spacego.GetSharedMeta(c.Cache, c.Network, entry.Record.Creator, share.MetaReference.RecordHash, share.MetaKey, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
				for index, reference := range entry.Record.Reference {
					if err := spacego.GetSharedFile(c.Cache, c.Network, entry.Record.Creator, reference.RecordHash, share.ChunkKey[index], func(entry *bcgo.BlockEntry, data []byte) error {
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
	aliases := aliasgo.OpenAndLoadAliasChannel(c.Cache, c.Network)
	files := spacego.OpenAndLoadFileChannel(node.Alias, c.Cache, c.Network)
	metas := spacego.OpenAndLoadMetaChannel(node.Alias, c.Cache, c.Network)
	return spacego.GetMeta(metas, c.Cache, node.Alias, node.Key, recordHash, func(entry *bcgo.BlockEntry, key []byte, meta *spacego.Meta) error {
		chunkKeys := make([][]byte, len(entry.Record.Reference))
		for index, reference := range entry.Record.Reference {
			if err := bcgo.ReadKey(files.GetHead(), nil, c.Cache, node.Alias, node.Key, reference.RecordHash, func(key []byte) error {
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
			shares := spacego.OpenAndLoadShareChannel(alias, c.Cache, c.Network)

			publicKey, err := aliases.GetPublicKey(c.Cache, alias)
			if err != nil {
				return err
			}
			acl := map[string]*rsa.PublicKey{
				alias:      publicKey,
				node.Alias: &node.Key.PublicKey,
			}
			if _, err := node.Write(shares, acl, nil, data); err != nil {
				return err
			}
			if _, _, err := node.Mine(shares, listener); err != nil {
				return err
			}
		}
		return nil
	})
}

// Search files owned by key
func (c *Client) Search(node *bcgo.Node, terms []string, callback MetaCallback) error {
	metas := spacego.OpenAndLoadMetaChannel(node.Alias, c.Cache, c.Network)
	if err := spacego.GetMeta(metas, c.Cache, node.Alias, node.Key, nil, func(metaEntry *bcgo.BlockEntry, metaKey []byte, meta *spacego.Meta) error {
		tags := spacego.OpenAndLoadTagChannel(base64.RawURLEncoding.EncodeToString(metaEntry.RecordHash), c.Cache, c.Network)
		return spacego.GetTag(tags, c.Cache, node.Alias, node.Key, nil, func(tagEntry *bcgo.BlockEntry, tagKey []byte, tag *spacego.Tag) error {
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
	shares := spacego.OpenAndLoadShareChannel(node.Alias, c.Cache, c.Network)
	if err := spacego.GetShare(shares, c.Cache, node.Alias, node.Key, nil, func(shareEntry *bcgo.BlockEntry, shareKey []byte, share *spacego.Share) error {
		if share.MetaReference == nil {
			// Meta reference not set
			return nil
		}
		if err := spacego.GetSharedMeta(c.Cache, c.Network, shareEntry.Record.Creator, share.MetaReference.RecordHash, share.MetaKey, func(metaEntry *bcgo.BlockEntry, meta *spacego.Meta) error {
			tags := spacego.OpenAndLoadTagChannel(base64.RawURLEncoding.EncodeToString(metaEntry.RecordHash), c.Cache, c.Network)
			return spacego.GetTag(tags, c.Cache, node.Alias, node.Key, nil, func(tagEntry *bcgo.BlockEntry, tagKey []byte, tag *spacego.Tag) error {
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
	metas := spacego.OpenAndLoadMetaChannel(node.Alias, c.Cache, c.Network)
	tags := spacego.OpenAndLoadTagChannel(node.Alias, c.Cache, c.Network)
	var references []*bcgo.Reference
	if err := spacego.GetMeta(metas, c.Cache, node.Alias, node.Key, recordHash, func(entry *bcgo.BlockEntry, key []byte, meta *spacego.Meta) error {
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
				ChannelName: metas.GetName(),
				RecordHash:  recordHash,
			}}
			reference, err := node.Write(tags, acl, references, data)
			if err != nil {
				return err
			}
			references = append(references, reference)
			if _, _, err := node.Mine(tags, listener); err != nil {
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
	metas := spacego.OpenAndLoadMetaChannel(node.Alias, c.Cache, c.Network)
	shares := spacego.OpenAndLoadShareChannel(node.Alias, c.Cache, c.Network)
	tags := spacego.OpenAndLoadTagChannel(node.Alias, c.Cache, c.Network)
	var references []*bcgo.Reference
	if err := spacego.GetShare(shares, c.Cache, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, share *spacego.Share) error {
		if share.MetaReference != nil && bytes.Equal(recordHash, share.MetaReference.RecordHash) {
			if err := spacego.GetSharedMeta(c.Cache, c.Network, entry.Record.Creator, recordHash, share.MetaKey, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
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
					reference, err := node.Write(tags, acl, []*bcgo.Reference{
						&bcgo.Reference{
							Timestamp:   entry.Record.Timestamp,
							ChannelName: metas.GetName(),
							RecordHash:  recordHash,
						},
					}, data)
					if err != nil {
						return err
					}
					references = append(references, reference)
					if _, _, err := node.Mine(tags, listener); err != nil {
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
	tags := spacego.OpenAndLoadTagChannel(node.Alias, c.Cache, c.Network)
	return spacego.GetTag(tags, c.Cache, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, tag *spacego.Tag) error {
		for _, reference := range entry.Record.Reference {
			if bytes.Equal(recordHash, reference.RecordHash) {
				callback(entry, tag)
			}
		}
		return nil
	})
}

func (c *Client) Handle(args []string) {
	if len(args) > 0 {
		switch args[0] {
		case "init":
			node, err := c.Init()
			if err != nil {
				log.Println(err)
				return
			}
			log.Println("Initialized")
			log.Println(node.Alias)
			publicKeyBytes, err := bcgo.RSAPublicKeyToPKIXBytes(&node.Key.PublicKey)
			if err != nil {
				log.Println(err)
				return
			}
			log.Println(base64.RawURLEncoding.EncodeToString(publicKeyBytes))
		case "add":
			if len(args) > 2 {
				node, err := bcgo.GetNode(c.Root, c.Cache, c.Network)
				if err != nil {
					log.Println(err)
					return
				}
				name := args[1]
				mime := args[2]
				// Read data from system in
				reader := os.Stdin
				if len(args) > 3 {
					// Read data from file
					file, err := os.Open(args[3])
					if err != nil {
						log.Println(err)
						return
					}
					reader = file
				} else {
					log.Println("Reading from stdin, use CTRL-D to terminate")
				}
				reference, err := c.Add(node, name, mime, reader)
				if err != nil {
					log.Println(err)
					return
				}
				log.Println("Mined metadata", base64.RawURLEncoding.EncodeToString(reference.RecordHash))
			} else {
				log.Println("add <name> <mime> <file>")
				log.Println("add <name> <mime> (data read from stdin)")
			}
		case "add-remote":
			if len(args) > 2 {
				node, err := bcgo.GetNode(c.Root, c.Cache, c.Network)
				if err != nil {
					log.Println(err)
					return
				}
				name := args[1]
				mime := args[2]
				// Read data from system in
				reader := os.Stdin
				if len(args) > 3 {
					// Read data from file
					file, err := os.Open(args[3])
					if err != nil {
						log.Println(err)
						return
					}
					reader = file
				} else {
					log.Println("Reading from stdin, use CTRL-D to terminate")
				}

				reference, err := c.AddRemote(node, name, mime, reader)
				if err != nil {
					log.Println(err)
					return
				}
				log.Println("Posted metadata", base64.RawURLEncoding.EncodeToString(reference.RecordHash))
			} else {
				log.Println("add-remote <name> <mime> <file>")
				log.Println("add-remote <name> <mime> (data read from stdin)")
			}
		case "list":
			node, err := bcgo.GetNode(c.Root, c.Cache, c.Network)
			if err != nil {
				log.Println(err)
				return
			}
			log.Println("Files:")
			count := 0
			if err := c.List(node, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
				count += 1
				return PrintMetaShort(os.Stdout, entry, meta)
			}); err != nil {
				log.Println(err)
				return
			}
			log.Println(count, "files")

			log.Println("Shared Files:")
			count = 0
			if err := c.ListShared(node, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
				count += 1
				return PrintMetaShort(os.Stdout, entry, meta)
			}); err != nil {
				log.Println(err)
				return
			}
			log.Println(count, "shared files")
		case "show":
			if len(args) > 1 {
				node, err := bcgo.GetNode(c.Root, c.Cache, c.Network)
				if err != nil {
					log.Println(err)
					return
				}
				recordHash, err := base64.RawURLEncoding.DecodeString(args[1])
				if err != nil {
					log.Println(err)
					return
				}
				success := false
				if err := c.Show(node, recordHash, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
					success = true
					return PrintMetaLong(os.Stdout, entry, meta)
				}); err != nil {
					log.Println(err)
					return
				}
				if !success {
					if err := c.ShowShared(node, recordHash, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
						return PrintMetaLong(os.Stdout, entry, meta)
					}); err != nil {
						log.Println(err)
						return
					}
				}
			} else {
				log.Println("show <file-hash>")
			}
		case "showall":
			if len(args) > 1 {
				node, err := bcgo.GetNode(c.Root, c.Cache, c.Network)
				if err != nil {
					log.Println(err)
					return
				}
				log.Println("Files:")
				count := 0
				if c.ShowAll(node, args[1], func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
					count += 1
					return PrintMetaShort(os.Stdout, entry, meta)
				}); err != nil {
					log.Println(err)
					return
				}
				log.Println(count, "files")

				log.Println("Shared Files:")
				count = 0
				if err = c.ShowAllShared(node, args[1], func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
					count += 1
					return PrintMetaShort(os.Stdout, entry, meta)
				}); err != nil {
					log.Println(err)
					return
				}
				log.Println(count, "shared files")
			} else {
				log.Println("showall <mime-type>")
			}
		case "get":
			if len(args) > 1 {
				node, err := bcgo.GetNode(c.Root, c.Cache, c.Network)
				if err != nil {
					log.Println(err)
					return
				}
				recordHash, err := base64.RawURLEncoding.DecodeString(args[1])
				if err != nil {
					log.Println(err)
					return
				}
				writer := os.Stdout
				if len(args) > 2 {
					log.Println("Writting to " + args[2])
					writer, err = os.OpenFile(args[2], os.O_CREATE|os.O_WRONLY, os.ModePerm)
					if err != nil {
						log.Println(err)
						return
					}
				}
				count, err := c.Get(node, recordHash, writer)
				if err != nil {
					log.Println(err)
					return
				}

				if count <= 0 {
					count, err = c.GetShared(node, recordHash, writer)
					if err != nil {
						log.Println(err)
						return
					}
				}

				log.Println("Wrote", bcgo.SizeToString(count))
			} else {
				log.Println("get <hash> <file>")
				log.Println("get <hash> (data written to stdout)")
			}
		case "share":
			if len(args) > 1 {
				node, err := bcgo.GetNode(c.Root, c.Cache, c.Network)
				if err != nil {
					log.Println(err)
					return
				}
				recordHash, err := base64.RawURLEncoding.DecodeString(args[1])
				if err != nil {
					log.Println(err)
					return
				}
				recipients := args[2:]
				if err := c.Share(node, &bcgo.PrintingMiningListener{os.Stdout}, recordHash, recipients); err != nil {
					log.Println(err)
					return
				}
			} else {
				log.Println("share <hash> <alias>... (share file with the given aliases)")
			}
		case "search":
			// search metas by tag
			if len(args) > 1 {
				ts := args[1:]
				log.Println("Searching Files for", ts)
				node, err := bcgo.GetNode(c.Root, c.Cache, c.Network)
				if err != nil {
					log.Println(err)
					return
				}
				log.Println("Files:")
				count := 0
				if c.Search(node, ts, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
					count += 1
					return PrintMetaShort(os.Stdout, entry, meta)
				}); err != nil {
					log.Println(err)
					return
				}
				log.Println(count, "files")

				log.Println("Shared Files:")
				count = 0
				if err = c.SearchShared(node, ts, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
					count += 1
					return PrintMetaShort(os.Stdout, entry, meta)
				}); err != nil {
					log.Println(err)
					return
				}
				log.Println(count, "shared files")
			} else {
				log.Println("search <tag>... (search files by tags)")
			}
		case "tag":
			if len(args) > 1 {
				node, err := bcgo.GetNode(c.Root, c.Cache, c.Network)
				if err != nil {
					log.Println(err)
					return
				}
				recordHash, err := base64.RawURLEncoding.DecodeString(args[1])
				if err != nil {
					log.Println(err)
					return
				}
				if len(args) > 2 {
					tags := args[2:]

					references, err := c.Tag(node, &bcgo.PrintingMiningListener{os.Stdout}, recordHash, tags)
					if err != nil {
						log.Println(err)
						return
					}

					if len(references) == 0 {
						references, err = c.TagShared(node, &bcgo.PrintingMiningListener{os.Stdout}, recordHash, tags)
						if err != nil {
							log.Println(err)
							return
						}
					}

					log.Println("Tagged", args[1], references)
				} else {
					if err := c.ShowTag(node, recordHash, func(entry *bcgo.BlockEntry, tag *spacego.Tag) {
						log.Println(tag.Value)
					}); err != nil {
						log.Println(err)
						return
					}
				}
			} else {
				log.Println("tag <hash> (display file tags)")
				log.Println("tag <hash> <tag>... (tag file with the given tags)")
			}
		default:
			log.Println("Cannot handle", args[0])
		}
	} else {
		PrintUsage(os.Stdout)
	}
}

func PrintUsage(output io.Writer) {
	fmt.Fprintln(output, "Space Usage:")
	fmt.Fprintln(output, "\tspace - display usage")
	fmt.Fprintln(output, "\tspace init - initializes environment, generates key pair, and registers alias")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "\tspace add [name] [type] - read stdin and mine a new record in blockchain")
	fmt.Fprintln(output, "\tspace add [name] [type] [file] - read file and mine a new record in blockchain")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "\tspace add-remote [name] [type] - read stdin and send a new record to Aletheia Ware's Remote Mining Service for mining into blockchain")
	fmt.Fprintln(output, "\tspace add-remote [name] [type] [file] - read file and send a new record to Aletheia Ware's Remote Mining Service for mining into blockchain")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "\tspace list - prints all files created by, or shared with, this key")
	fmt.Fprintln(output, "\tspace show [hash] - display metadata of file with given hash")
	fmt.Fprintln(output, "\tspace showall [type] - display metadata of all files with given MIME type")
	fmt.Fprintln(output, "\tspace get [hash] - write file with given hash to stdout")
	fmt.Fprintln(output, "\tspace get [hash] [file] - write file with given hash to file")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "\tspace share [hash] [alias]... - shares file with given hash with given aliases")
	fmt.Fprintln(output, "\tspace tag [hash] [tag]... - tags file with given hash with given tags")
	fmt.Fprintln(output, "\tspace search [tag]... - search files for given tags")
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Get root directory
	rootDir, err := bcgo.GetRootDirectory()
	if err != nil {
		log.Fatal("Could not get root directory:", err)
	}
	log.Println("Root Directory:", rootDir)

	// Get cache directory
	cacheDir, err := bcgo.GetCacheDirectory(rootDir)
	if err != nil {
		log.Fatal("Could not get cache directory:", err)
	}
	log.Println("Cache Directory:", cacheDir)

	// Create file cache
	cache, err := bcgo.NewFileCache(cacheDir)
	if err != nil {
		log.Fatal("Could not create file cache:", err)
	}

	// Get peers
	peers, err := bcgo.GetPeers(rootDir)
	if err != nil {
		log.Fatal("Could not get network peers:", err)
	}

	// Create network of peers
	network := &bcgo.TcpNetwork{peers}

	client := &Client{
		Root:    rootDir,
		Cache:   cache,
		Network: network,
	}

	client.Handle(os.Args[1:])
}

func PrintMetaShort(output io.Writer, entry *bcgo.BlockEntry, meta *spacego.Meta) error {
	hash := base64.RawURLEncoding.EncodeToString(entry.RecordHash)
	timestamp := bcgo.TimestampToString(entry.Record.Timestamp)
	size := bcgo.SizeToString(meta.Size)
	fmt.Fprintf(output, "%s %s %s %s %s\n", hash, timestamp, meta.Name, meta.Type, size)
	return nil
}

func PrintMetaLong(output io.Writer, entry *bcgo.BlockEntry, meta *spacego.Meta) error {
	fmt.Fprintf(output, "Hash: %s\n", base64.RawURLEncoding.EncodeToString(entry.RecordHash))
	fmt.Fprintf(output, "Timestamp: %s\n", bcgo.TimestampToString(entry.Record.Timestamp))
	fmt.Fprintf(output, "Name: %s\n", meta.Name)
	fmt.Fprintf(output, "Type: %s\n", meta.Type)
	fmt.Fprintf(output, "Size: %s\n", bcgo.SizeToString(meta.Size))
	fmt.Fprintf(output, "Chunks: %d\n", len(entry.Record.Reference))
	for index, reference := range entry.Record.Reference {
		fmt.Fprintf(output, "\t%d: %s\n", index, base64.RawURLEncoding.EncodeToString(reference.RecordHash))
	}
	return nil
}
