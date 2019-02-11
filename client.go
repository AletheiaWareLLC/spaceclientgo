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
	"github.com/AletheiaWareLLC/aliasgo"
	"github.com/AletheiaWareLLC/bcgo"
	"github.com/AletheiaWareLLC/financego"
	"github.com/AletheiaWareLLC/spacego"
	"github.com/golang/protobuf/proto"
	"log"
	"os"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	if len(os.Args) > 1 {
		// Handle Arguments
		switch os.Args[1] {
		case "init":
			if err := bcgo.AddPeer(aliasgo.ALIAS, bcgo.BC_HOST); err != nil {
				log.Println(err)
				return
			}
			if err := bcgo.AddPeer(financego.CHARGE, spacego.SPACE_HOST); err != nil {
				log.Println(err)
				return
			}
			if err := bcgo.AddPeer(financego.CUSTOMER, spacego.SPACE_HOST); err != nil {
				log.Println(err)
				return
			}
			if err := bcgo.AddPeer(financego.SUBSCRIPTION, spacego.SPACE_HOST); err != nil {
				log.Println(err)
				return
			}
			if err := bcgo.AddPeer(financego.USAGE_RECORD, spacego.SPACE_HOST); err != nil {
				log.Println(err)
				return
			}
			aliases, err := aliasgo.OpenAliasChannel()
			if err != nil {
				log.Println(err)
				return
			}
			node, err := bcgo.GetNode()
			if err != nil {
				log.Println(err)
				return
			}
			alias, err := aliasgo.RegisterAlias(aliases, node.Alias, node.Key)
			if err != nil {
				log.Println(err)
				return
			}
			log.Println(alias)
			publicKeyBytes, err := bcgo.RSAPublicKeyToPKIXBytes(&node.Key.PublicKey)
			if err != nil {
				log.Println(err)
				return
			}
			log.Println(base64.RawURLEncoding.EncodeToString(publicKeyBytes))
			log.Println("Initialized")
			if err := bcgo.AddPeer(spacego.SPACE_PREFIX_FILE+alias, spacego.SPACE_HOST); err != nil {
				log.Println(err)
				return
			}
			if err := bcgo.AddPeer(spacego.SPACE_PREFIX_META+alias, spacego.SPACE_HOST); err != nil {
				log.Println(err)
				return
			}
			if err := bcgo.AddPeer(spacego.SPACE_PREFIX_SHARE+alias, spacego.SPACE_HOST); err != nil {
				log.Println(err)
				return
			}
			if err := bcgo.AddPeer(spacego.SPACE_PREFIX_TAG+alias, spacego.SPACE_HOST); err != nil {
				log.Println(err)
				return
			}
		case "list":
			node, err := bcgo.GetNode()
			if err != nil {
				log.Println(err)
				return
			}
			metas, err := bcgo.OpenChannel(spacego.SPACE_PREFIX_META + node.Alias)
			if err != nil {
				log.Println(err)
				return
			}
			count := 0
			// List files owned by key
			err = spacego.GetMeta(metas, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, meta *spacego.Meta) error {
				hash := base64.RawURLEncoding.EncodeToString(entry.RecordHash)
				timestamp := bcgo.TimestampToString(entry.Record.Timestamp)
				size := bcgo.SizeToString(meta.Size)
				log.Println(hash, timestamp, meta.Name, size, meta.Type)
				count = count + 1
				return nil
			})
			if err != nil {
				log.Println(err)
				return
			}
			log.Println(count, "files")
			shares, err := bcgo.OpenChannel(spacego.SPACE_PREFIX_SHARE + node.Alias)
			if err != nil {
				log.Println(err)
				return
			}
			count = 0
			// List files shared with key
			err = spacego.GetShare(shares, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, share *spacego.Share) error {
				hash := base64.RawURLEncoding.EncodeToString(entry.RecordHash)
				timestamp := bcgo.TimestampToString(entry.Record.Timestamp)
				log.Println("Share", hash, timestamp)
				for index, reference := range entry.Record.Reference {
					metas, err := bcgo.OpenChannel(spacego.SPACE_PREFIX_META + entry.Record.Creator)
					if err != nil {
						return err
					}
					err = spacego.GetSharedMeta(metas, reference.RecordHash, share.MetaKey, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
						size := bcgo.SizeToString(meta.Size)
						log.Println(index, hash, timestamp, meta.Name, size, meta.Type)
						count = count + 1
						return nil
					})
					if err != nil {
						return err
					}
				}
				return nil
			})
			if err != nil {
				log.Println(err)
				return
			}
			log.Println(count, "shared files")
		case "show":
			// Show file owned by key with given hash
			if len(os.Args) > 2 {
				recordHash, err := base64.RawURLEncoding.DecodeString(os.Args[2])
				if err != nil {
					log.Println(err)
					return
				}
				node, err := bcgo.GetNode()
				if err != nil {
					log.Println(err)
					return
				}
				metas, err := bcgo.OpenChannel(spacego.SPACE_PREFIX_META + node.Alias)
				if err != nil {
					log.Println(err)
					return
				}
				success := false
				err = spacego.GetMeta(metas, node.Alias, node.Key, recordHash, func(entry *bcgo.BlockEntry, key []byte, meta *spacego.Meta) error {
					success = true
					hash := base64.RawURLEncoding.EncodeToString(entry.RecordHash)
					timestamp := bcgo.TimestampToString(entry.Record.Timestamp)
					size := bcgo.SizeToString(meta.Size)
					log.Println("Hash:", hash)
					log.Println("Timestamp:", timestamp)
					log.Println("Name:", meta.Name)
					log.Println("Type:", meta.Type)
					log.Println("Size:", size)
					log.Println("Chunks:")
					for index, reference := range entry.Record.Reference {
						hash := base64.RawURLEncoding.EncodeToString(reference.RecordHash)
						log.Println("\t", index, hash)
					}
					return nil
				})
				if err != nil {
					log.Println(err)
					return
				}
				if !success {
					// Show file shared to key with given hash
					shares, err := bcgo.OpenChannel(spacego.SPACE_PREFIX_SHARE + node.Alias)
					if err != nil {
						log.Println(err)
						return
					}
					err = spacego.GetShare(shares, node.Alias, node.Key, recordHash, func(entry *bcgo.BlockEntry, key []byte, share *spacego.Share) error {
						hash := base64.RawURLEncoding.EncodeToString(entry.RecordHash)
						timestamp := bcgo.TimestampToString(entry.Record.Timestamp)
						log.Println("Share", hash, timestamp)
						for index, reference := range entry.Record.Reference {
							metas, err := bcgo.OpenChannel(spacego.SPACE_PREFIX_META + entry.Record.Creator)
							if err != nil {
								return err
							}
							log.Println(index)
							err = spacego.GetSharedMeta(metas, reference.RecordHash, share.MetaKey, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
								hash := base64.RawURLEncoding.EncodeToString(entry.RecordHash)
								timestamp := bcgo.TimestampToString(entry.Record.Timestamp)
								size := bcgo.SizeToString(meta.Size)
								log.Println("Hash:", hash)
								log.Println("Timestamp:", timestamp)
								log.Println("Name:", meta.Name)
								log.Println("Type:", meta.Type)
								log.Println("Size:", size)
								log.Println("Chunks:")
								for index, reference := range entry.Record.Reference {
									hash := base64.RawURLEncoding.EncodeToString(reference.RecordHash)
									log.Println("\t", index, hash)
								}
								return nil
							})
							if err != nil {
								return err
							}
						}
						return nil
					})
					if err != nil {
						log.Println(err)
						return
					}
				}
			} else {
				log.Println("show <file-hash>")
			}
		case "showall":
			// Show all files owned by key with given mime-type
			if len(os.Args) > 2 {
				node, err := bcgo.GetNode()
				if err != nil {
					log.Println(err)
					return
				}
				metas, err := bcgo.OpenChannel(spacego.SPACE_PREFIX_META + node.Alias)
				if err != nil {
					log.Println(err)
					return
				}
				count := 0
				err = spacego.GetMeta(metas, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, meta *spacego.Meta) error {
					if meta.Type == os.Args[2] {
						hash := base64.RawURLEncoding.EncodeToString(entry.RecordHash)
						timestamp := bcgo.TimestampToString(entry.Record.Timestamp)
						size := bcgo.SizeToString(meta.Size)
						log.Println(hash, timestamp, meta.Name, size)
						count = count + 1
					}
					return nil
				})
				if err != nil {
					log.Println(err)
					return
				}
				log.Println(count, "files")
				shares, err := bcgo.OpenChannel(spacego.SPACE_PREFIX_SHARE + node.Alias)
				if err != nil {
					log.Println(err)
					return
				}
				count = 0
				// Show all files shared to key with given mime-type
				err = spacego.GetShare(shares, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, share *spacego.Share) error {
					for _, reference := range entry.Record.Reference {
						metas, err := bcgo.OpenChannel(spacego.SPACE_PREFIX_META + entry.Record.Creator)
						if err != nil {
							return err
						}
						err = spacego.GetSharedMeta(metas, reference.RecordHash, share.MetaKey, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
							hash := base64.RawURLEncoding.EncodeToString(entry.RecordHash)
							timestamp := bcgo.TimestampToString(entry.Record.Timestamp)
							size := bcgo.SizeToString(meta.Size)
							log.Println(hash, timestamp, meta.Name, size)
							count = count + 1
							return nil
						})
						if err != nil {
							return err
						}
					}
					return nil
				})
				if err != nil {
					log.Println(err)
					return
				}
				log.Println(count, "shared files")
			} else {
				log.Println("showall <mime-type>")
			}
		case "download":
			// Download file by given hash
			if len(os.Args) > 2 {
				recordHash, err := base64.RawURLEncoding.DecodeString(os.Args[2])
				if err != nil {
					log.Println(err)
					return
				}
				writer := os.Stdout
				if len(os.Args) > 3 {
					writer, err = os.OpenFile(os.Args[3], os.O_CREATE|os.O_WRONLY, os.ModePerm)
					if err != nil {
						log.Println(err)
						return
					}
				}
				node, err := bcgo.GetNode()
				if err != nil {
					log.Println(err)
					return
				}
				files, err := bcgo.OpenChannel(spacego.SPACE_PREFIX_FILE + node.Alias)
				if err != nil {
					log.Println(err)
					return
				}
				metas, err := bcgo.OpenChannel(spacego.SPACE_PREFIX_META + node.Alias)
				if err != nil {
					log.Println(err)
					return
				}
				success := false
				err = spacego.GetMeta(metas, node.Alias, node.Key, recordHash, func(entry *bcgo.BlockEntry, key []byte, meta *spacego.Meta) error {
					success = true
					count := 0
					for _, reference := range entry.Record.Reference {
						err := spacego.GetFile(files, node.Alias, node.Key, reference.RecordHash, func(entry *bcgo.BlockEntry, key, data []byte) error {
							c, err := writer.Write(data)
							if err != nil {
								return err
							}
							count = count + c
							return nil
						})
						if err != nil {
							return err
						}
						log.Println("Downloaded", uint64(count*100)/meta.Size, "%", count, "/", meta.Size)
					}
					return nil
				})
				if err != nil {
					log.Println(err)
					return
				}
				if !success {
					// Download file shared to key with given hash
					shares, err := bcgo.OpenChannel(spacego.SPACE_PREFIX_SHARE + node.Alias)
					if err != nil {
						log.Println(err)
						return
					}
					err = spacego.GetShare(shares, node.Alias, node.Key, recordHash, func(entry *bcgo.BlockEntry, key []byte, share *spacego.Share) error {
						for _, reference := range entry.Record.Reference {
							files, err := bcgo.OpenChannel(spacego.SPACE_PREFIX_FILE + entry.Record.Creator)
							if err != nil {
								return err
							}
							metas, err := bcgo.OpenChannel(spacego.SPACE_PREFIX_META + entry.Record.Creator)
							if err != nil {
								return err
							}
							err = spacego.GetSharedMeta(metas, reference.RecordHash, share.MetaKey, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
								count := 0
								for index, reference := range entry.Record.Reference {
									err := spacego.GetSharedFile(files, reference.RecordHash, share.ChunkKey[index], func(entry *bcgo.BlockEntry, data []byte) error {
										c, err := writer.Write(data)
										if err != nil {
											return err
										}
										count = count + c
										return nil
									})
									if err != nil {
										return err
									}
									log.Println("Downloaded", uint64(count*100)/meta.Size, "%", count, "/", meta.Size)
								}
								return nil
							})
							if err != nil {
								return err
							}
						}
						return nil
					})
					if err != nil {
						log.Println(err)
						return
					}
				}
				if len(os.Args) > 3 {
					log.Println("Downloaded to " + os.Args[3])
				}
			} else {
				log.Println("download <hash> <file>")
				log.Println("download <hash> (data written to stdout)")
			}
		case "upload":
			if len(os.Args) > 3 {
				name := os.Args[2]
				mime := os.Args[3]
				// Read data from system in
				reader := os.Stdin
				if len(os.Args) > 4 {
					// Read data from file
					file, err := os.Open(os.Args[4])
					if err != nil {
						log.Println(err)
						return
					}
					reader = file
				}

				// TODO compress data

				node, err := bcgo.GetNode()
				if err != nil {
					log.Println(err)
					return
				}

				acl := map[string]*rsa.PublicKey{
					node.Alias: &node.Key.PublicKey,
				}

				var references []*bcgo.Reference

				index := 0
				size, err := bcgo.CreateRecords(node.Alias, node.Key, acl, references, reader, func(key []byte, record *bcgo.Record) error {
					log.Println("Uploading chunk", index)
					index = index + 1
					reference, err := spacego.UploadRecord("file", record)
					if err != nil {
						return err
					}
					log.Println("Uploaded", base64.RawURLEncoding.EncodeToString(reference.RecordHash))
					references = append(references, reference)
					return nil
				})
				if err != nil {
					log.Println(err)
					return
				}

				log.Println("Uploaded", name, "in", len(references), "chunks")

				meta := spacego.Meta{
					Name: name,
					Size: uint64(size),
					Type: mime,
				}

				data, err := proto.Marshal(&meta)
				if err != nil {
					log.Println(err)
					return
				}

				_, record, err := bcgo.CreateRecord(node.Alias, node.Key, acl, references, data)
				if err != nil {
					log.Println(err)
					return
				}

				reference, err := spacego.UploadRecord("meta", record)
				if err != nil {
					log.Println(err)
					return
				}
				log.Println("Uploaded metadata", base64.RawURLEncoding.EncodeToString(reference.RecordHash))
			} else {
				log.Println("upload <name> <mime> <file>")
				log.Println("upload <name> <mime> (data read from stdin)")
			}
		case "customer":
			node, err := bcgo.GetNode()
			if err != nil {
				log.Println(err)
				return
			}
			customers, err := financego.OpenCustomerChannel()
			if err != nil {
				log.Println(err)
				return
			}
			customer, err := financego.GetCustomerSync(customers, node.Alias, node.Key, node.Alias)
			if err != nil {
				log.Println(err)
				return
			}
			log.Println(customer)
		case "subscription":
			node, err := bcgo.GetNode()
			if err != nil {
				log.Println(err)
				return
			}
			subscriptions, err := financego.OpenSubscriptionChannel()
			if err != nil {
				log.Println(err)
				return
			}
			subscription, err := financego.GetSubscriptionSync(subscriptions, node.Alias, node.Key, node.Alias)
			if err != nil {
				publicKeyBytes, err := bcgo.RSAPublicKeyToPKIXBytes(&node.Key.PublicKey)
				if err != nil {
					log.Println(err)
					return
				}
				log.Println(err)
				log.Println("To subscribe for remote mining, visit", spacego.SPACE_WEBSITE+"/subscription?alias="+node.Alias, "and")
				log.Println("enter your alias, email, payment info, and public key:")
				log.Println(base64.RawURLEncoding.EncodeToString(publicKeyBytes))
			} else {
				log.Println(subscription)
			}
		case "stripe-webhook":
			// TODO
		case "share":
			if len(os.Args) > 2 {
				hash, err := base64.RawURLEncoding.DecodeString(os.Args[2])
				if err != nil {
					log.Println(err)
					return
				}
				recipients := os.Args[3:]
				node, err := bcgo.GetNode()
				if err != nil {
					log.Println(err)
					return
				}
				aliases, err := aliasgo.OpenAliasChannel()
				if err != nil {
					log.Println(err)
					return
				}
				files, err := bcgo.OpenChannel(spacego.SPACE_PREFIX_FILE + node.Alias)
				if err != nil {
					log.Println(err)
					return
				}
				metas, err := bcgo.OpenChannel(spacego.SPACE_PREFIX_META + node.Alias)
				if err != nil {
					log.Println(err)
					return
				}
				err = spacego.GetMeta(metas, node.Alias, node.Key, hash, func(entry *bcgo.BlockEntry, key []byte, meta *spacego.Meta) error {
					chunkKeys := make([][]byte, len(entry.Record.Reference))
					for index, reference := range entry.Record.Reference {
						// TODO for each chunk referenced in meta, decrypt key
						err := files.GetKey(node.Alias, node.Key, reference.RecordHash, func(key []byte) error {
							chunkKeys[index] = key
							return nil
						})
						if err != nil {
							return err
						}
					}
					share := spacego.Share{
						MetaKey:  key,
						ChunkKey: chunkKeys,
					}
					data, err := proto.Marshal(&share)
					if err != nil {
						return err
					}

					references := []*bcgo.Reference{&bcgo.Reference{
						ChannelName: metas.Name,
						RecordHash:  hash,
					}}

					for _, alias := range recipients {
						shares, err := bcgo.OpenChannel(spacego.SPACE_PREFIX_SHARE + alias)
						if err != nil {
							return err
						}

						publicKey, err := aliasgo.GetPublicKey(aliases, alias)
						if err != nil {
							return err
						}
						acl := map[string]*rsa.PublicKey{
							alias:      publicKey,
							node.Alias: &node.Key.PublicKey,
						}
						reference, err := node.Mine(shares, acl, references, data)
						if err != nil {
							return err
						}
						log.Println("Shared", alias, base64.RawURLEncoding.EncodeToString(reference.BlockHash), base64.RawURLEncoding.EncodeToString(reference.RecordHash))
					}
					return nil
				})
				if err != nil {
					log.Println(err)
					return
				}
			} else {
				log.Println("share <hash> <alias>... (share file with the given aliases)")
			}
		case "search":
			// search metas by tag
			if len(os.Args) > 2 {
				node, err := bcgo.GetNode()
				if err != nil {
					log.Println(err)
					return
				}
				metas, err := bcgo.OpenChannel(spacego.SPACE_PREFIX_META + node.Alias)
				if err != nil {
					log.Println(err)
					return
				}
				tags, err := bcgo.OpenChannel(spacego.SPACE_PREFIX_TAG + node.Alias)
				if err != nil {
					log.Println(err)
					return
				}
				ts := os.Args[2:]
				log.Println("Searching", ts)
				err = spacego.GetTag(tags, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, tag *spacego.Tag) error {
					for _, value := range ts {
						if tag.Value == value {
							for _, reference := range entry.Record.Reference {
								log.Println(base64.RawURLEncoding.EncodeToString(reference.RecordHash))
								err = spacego.GetMeta(metas, node.Alias, node.Key, reference.RecordHash, func(entry *bcgo.BlockEntry, key []byte, meta *spacego.Meta) error {
									log.Println(meta)
									return nil
								})
								if err != nil {
									return err
								}
							}
						}
					}
					return nil
				})
				if err != nil {
					log.Println(err)
					return
				}
			} else {
				log.Println("search <tag>... (search files by tags)")
			}
		case "tag":
			// display meta tags
			// add tags to meta
			if len(os.Args) > 2 {
				hash, err := base64.RawURLEncoding.DecodeString(os.Args[2])
				if err != nil {
					log.Println(err)
					return
				}
				node, err := bcgo.GetNode()
				if err != nil {
					log.Println(err)
					return
				}
				metas, err := bcgo.OpenChannel(spacego.SPACE_PREFIX_META + node.Alias)
				if err != nil {
					log.Println(err)
					return
				}
				tags, err := bcgo.OpenChannel(spacego.SPACE_PREFIX_TAG + node.Alias)
				if err != nil {
					log.Println(err)
					return
				}
				err = spacego.GetMeta(metas, node.Alias, node.Key, hash, func(entry *bcgo.BlockEntry, key []byte, meta *spacego.Meta) error {
					log.Println(meta)
					if len(os.Args) > 3 {
						ts := os.Args[3:]
						log.Println("Tagging with", ts)
						for _, t := range ts {
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
								ChannelName: metas.Name,
								RecordHash:  hash,
							}}
							reference, err := node.Mine(tags, acl, references, data)
							if err != nil {
								return err
							}
							log.Println("Tagged", os.Args[1], t, base64.RawURLEncoding.EncodeToString(reference.RecordHash))
						}
					} else {
						log.Println("Displaying tags for", meta)
						err := spacego.GetTag(tags, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, tag *spacego.Tag) error {
							for _, reference := range entry.Record.Reference {
								if bytes.Equal(hash, reference.RecordHash) {
									log.Println(tag.Value)
								}
							}
							return nil
						})
						if err != nil {
							return err
						}
					}
					return nil
				})
				if err != nil {
					log.Println(err)
					return
				}
			} else {
				log.Println("tag <hash> (display file tags)")
				log.Println("tag <hash> <tag>... (tag file with the given tags)")
			}
		default:
			if len(os.Args) > 2 {
				name := os.Args[1]
				mime := os.Args[2]
				// Read data from system in
				reader := os.Stdin
				if len(os.Args) > 3 {
					// Read data from file
					file, err := os.Open(os.Args[3])
					if err != nil {
						log.Println(err)
						return
					}
					reader = file
				}

				// TODO compress data

				node, err := bcgo.GetNode()
				if err != nil {
					log.Println(err)
					return
				}

				files, err := bcgo.OpenChannel(spacego.SPACE_PREFIX_FILE + node.Alias)
				if err != nil {
					log.Println(err)
					return
				}

				metas, err := bcgo.OpenChannel(spacego.SPACE_PREFIX_META + node.Alias)
				if err != nil {
					log.Println(err)
					return
				}

				acl := map[string]*rsa.PublicKey{
					node.Alias: &node.Key.PublicKey,
				}

				var references []*bcgo.Reference

				index := 0
				size, err := bcgo.CreateRecords(node.Alias, node.Key, acl, references, reader, func(key []byte, record *bcgo.Record) error {
					log.Println("Mining chunk", index)
					index = index + 1
					reference, err := node.MineRecord(files, record)
					if err != nil {
						return err
					}
					log.Println("Mined", base64.RawURLEncoding.EncodeToString(reference.RecordHash))
					references = append(references, reference)
					return nil
				})
				if err != nil {
					log.Println(err)
					return
				}

				log.Println("Mined", name, "in", len(references), "chunks")

				// TODO Add preview

				meta := spacego.Meta{
					Name: name,
					Size: uint64(size),
					Type: mime,
				}

				data, err := proto.Marshal(&meta)
				if err != nil {
					log.Println(err)
					return
				}

				reference, err := node.Mine(metas, acl, references, data)
				if err != nil {
					log.Println(err)
					return
				}

				log.Println("Mined metadata", base64.RawURLEncoding.EncodeToString(reference.RecordHash))
			} else {
				bcgo.GetAndPrintURL(spacego.SPACE_WEBSITE)
			}
		}
	} else {
		bcgo.GetAndPrintURL(spacego.SPACE_WEBSITE)
	}
}
