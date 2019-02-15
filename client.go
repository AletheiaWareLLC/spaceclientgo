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
			if err := bcgo.AddPeer(spacego.SPACE_HOST); err != nil {
				log.Println(err)
				return
			}
			if err := bcgo.AddPeer(bcgo.BC_HOST); err != nil {
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
		case "list":
			node, err := bcgo.GetNode()
			if err != nil {
				log.Println(err)
				return
			}
			metas, err := bcgo.OpenAndSyncChannel(spacego.SPACE_PREFIX_META + node.Alias)
			if err != nil {
				log.Println(err)
				return
			}
			log.Println("Files:")
			count := 0
			// List files owned by key
			err = spacego.GetMeta(metas, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, meta *spacego.Meta) error {
				count = count + 1
				return ShowFileShort(entry, meta)
			})
			if err != nil {
				log.Println(err)
				return
			}
			log.Println(count, "files")
			shares, err := bcgo.OpenAndSyncChannel(spacego.SPACE_PREFIX_SHARE + node.Alias)
			if err != nil {
				log.Println(err)
				return
			}
			log.Println("Shared Files:")
			count = 0
			// List files shared with key
			err = spacego.GetShare(shares, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, share *spacego.Share) error {
				for _, reference := range entry.Record.Reference {
					err = spacego.GetSharedMeta(entry.Record.Creator, reference.RecordHash, share.MetaKey, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
						count = count + 1
						return ShowFileShort(entry, meta)
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
				metas, err := bcgo.OpenAndSyncChannel(spacego.SPACE_PREFIX_META + node.Alias)
				if err != nil {
					log.Println(err)
					return
				}
				success := false
				err = spacego.GetMeta(metas, node.Alias, node.Key, recordHash, func(entry *bcgo.BlockEntry, key []byte, meta *spacego.Meta) error {
					success = true
					return ShowFileLong(entry, meta)
				})
				if err != nil {
					log.Println(err)
					return
				}
				if !success {
					// Show file shared to key with given hash
					shares, err := bcgo.OpenAndSyncChannel(spacego.SPACE_PREFIX_SHARE + node.Alias)
					if err != nil {
						log.Println(err)
						return
					}
					err = spacego.GetShare(shares, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, share *spacego.Share) error {
						for _, reference := range entry.Record.Reference {
							if bytes.Equal(recordHash, reference.RecordHash) {
								err = spacego.GetSharedMeta(entry.Record.Creator, recordHash, share.MetaKey, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
									return ShowFileLong(entry, meta)
								})
								if err != nil {
									return err
								}
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
				metas, err := bcgo.OpenAndSyncChannel(spacego.SPACE_PREFIX_META + node.Alias)
				if err != nil {
					log.Println(err)
					return
				}
				log.Println("Files:")
				count := 0
				err = spacego.GetMeta(metas, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, meta *spacego.Meta) error {
					if meta.Type == os.Args[2] {
						count = count + 1
						return ShowFileShort(entry, meta)
					}
					return nil
				})
				if err != nil {
					log.Println(err)
					return
				}
				log.Println(count, "files")
				shares, err := bcgo.OpenAndSyncChannel(spacego.SPACE_PREFIX_SHARE + node.Alias)
				if err != nil {
					log.Println(err)
					return
				}
				log.Println("Shared Files:")
				count = 0
				// Show all files shared to key with given mime-type
				err = spacego.GetShare(shares, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, share *spacego.Share) error {
					for _, reference := range entry.Record.Reference {
						err = spacego.GetSharedMeta(entry.Record.Creator, reference.RecordHash, share.MetaKey, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
							if meta.Type == os.Args[2] {
								count = count + 1
								return ShowFileShort(entry, meta)
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
				log.Println(count, "shared files")
			} else {
				log.Println("showall <mime-type>")
			}
		case "get":
			// Get file by given hash
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
				files, err := bcgo.OpenAndSyncChannel(spacego.SPACE_PREFIX_FILE + node.Alias)
				if err != nil {
					log.Println(err)
					return
				}
				metas, err := bcgo.OpenAndSyncChannel(spacego.SPACE_PREFIX_META + node.Alias)
				if err != nil {
					log.Println(err)
					return
				}
				success := false
				err = spacego.GetMeta(metas, node.Alias, node.Key, recordHash, func(entry *bcgo.BlockEntry, key []byte, meta *spacego.Meta) error {
					success = true
					for _, reference := range entry.Record.Reference {
						err := spacego.GetFile(files, node.Alias, node.Key, reference.RecordHash, func(entry *bcgo.BlockEntry, key, data []byte) error {
							_, err := writer.Write(data)
							if err != nil {
								return err
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
				if !success {
					// Get file shared to key with given hash
					shares, err := bcgo.OpenAndSyncChannel(spacego.SPACE_PREFIX_SHARE + node.Alias)
					if err != nil {
						log.Println(err)
						return
					}
					err = spacego.GetShare(shares, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, share *spacego.Share) error {
						for _, reference := range entry.Record.Reference {
							if bytes.Equal(recordHash, reference.RecordHash) {
								err = spacego.GetSharedMeta(entry.Record.Creator, recordHash, share.MetaKey, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
									for index, reference := range entry.Record.Reference {
										err := spacego.GetSharedFile(entry.Record.Creator, reference.RecordHash, share.ChunkKey[index], func(entry *bcgo.BlockEntry, data []byte) error {
											_, err := writer.Write(data)
											if err != nil {
												return err
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
									return err
								}
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
					log.Println("Written to " + os.Args[3])
				}
			} else {
				log.Println("get <hash> <file>")
				log.Println("get <hash> (data written to stdout)")
			}
		case "post":
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
				} else {
					log.Println("Reading from stdin, use CTRL-D to terminate")
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
					index = index + 1
					reference, err := spacego.PostRecord("file", record)
					if err != nil {
						return err
					}
					log.Println("Posted", base64.RawURLEncoding.EncodeToString(reference.RecordHash))
					references = append(references, reference)
					return nil
				})
				if err != nil {
					log.Println(err)
					return
				}

				log.Println("Posted", name, "in", len(references), "chunks")

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

				reference, err := spacego.PostRecord("meta", record)
				if err != nil {
					log.Println(err)
					return
				}
				log.Println("Posted metadata", base64.RawURLEncoding.EncodeToString(reference.RecordHash))
			} else {
				log.Println("post <name> <mime> <file>")
				log.Println("post <name> <mime> (data read from stdin)")
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
				recordHash, err := base64.RawURLEncoding.DecodeString(os.Args[2])
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
				metas, err := bcgo.OpenAndSyncChannel(spacego.SPACE_PREFIX_META + node.Alias)
				if err != nil {
					log.Println(err)
					return
				}
				err = spacego.GetMeta(metas, node.Alias, node.Key, recordHash, func(entry *bcgo.BlockEntry, key []byte, meta *spacego.Meta) error {
					log.Println("Sharing", meta.Name, "with", recipients)
					chunkKeys := make([][]byte, len(entry.Record.Reference))
					for index, reference := range entry.Record.Reference {
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
						RecordHash:  recordHash,
					}}

					for _, alias := range recipients {
						shares, err := bcgo.OpenAndSyncChannel(spacego.SPACE_PREFIX_SHARE + alias)
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
				metas, err := bcgo.OpenAndSyncChannel(spacego.SPACE_PREFIX_META + node.Alias)
				if err != nil {
					log.Println(err)
					return
				}
				shares, err := bcgo.OpenAndSyncChannel(spacego.SPACE_PREFIX_SHARE + node.Alias)
				if err != nil {
					log.Println(err)
					return
				}
				tags, err := bcgo.OpenAndSyncChannel(spacego.SPACE_PREFIX_TAG + node.Alias)
				if err != nil {
					log.Println(err)
					return
				}
				ts := os.Args[2:]
				log.Println("Searching Files for", ts)
				count := 0
				err = spacego.GetTag(tags, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, tag *spacego.Tag) error {
					for _, value := range ts {
						if tag.Value == value {
							for _, reference := range entry.Record.Reference {
								err = spacego.GetMeta(metas, node.Alias, node.Key, reference.RecordHash, func(entry *bcgo.BlockEntry, key []byte, meta *spacego.Meta) error {
									count = count + 1
									return ShowFileShort(entry, meta)
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
				log.Println(count, "files")

				log.Println("Searching Shared Files for", ts)
				count = 0
				err = spacego.GetTag(tags, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, tag *spacego.Tag) error {
					for _, value := range ts {
						if tag.Value == value {
							for _, reference := range entry.Record.Reference {
								recordHash := reference.RecordHash
								err = spacego.GetShare(shares, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, share *spacego.Share) error {
									for _, reference := range entry.Record.Reference {
										if bytes.Equal(recordHash, reference.RecordHash) {
											err = spacego.GetSharedMeta(entry.Record.Creator, recordHash, share.MetaKey, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
												count = count + 1
												return ShowFileShort(entry, meta)
											})
											if err != nil {
												return err
											}
										}
									}
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
				log.Println(count, "shared files")
			} else {
				log.Println("search <tag>... (search files by tags)")
			}
		case "tag":
			// display meta tags
			// add tags to meta
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
				metas, err := bcgo.OpenAndSyncChannel(spacego.SPACE_PREFIX_META + node.Alias)
				if err != nil {
					log.Println(err)
					return
				}
				tags, err := bcgo.OpenAndSyncChannel(spacego.SPACE_PREFIX_TAG + node.Alias)
				if err != nil {
					log.Println(err)
					return
				}
				success := false
				err = spacego.GetMeta(metas, node.Alias, node.Key, recordHash, func(entry *bcgo.BlockEntry, key []byte, meta *spacego.Meta) error {
					success = true
					if len(os.Args) > 3 {
						ts := os.Args[3:]
						log.Println("Tagging", meta.Name, "with", ts)
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
								RecordHash:  recordHash,
							}}
							reference, err := node.Mine(tags, acl, references, data)
							if err != nil {
								return err
							}
							log.Println("Tagged", os.Args[2], t, base64.RawURLEncoding.EncodeToString(reference.RecordHash))
						}
					} else {
						log.Println("Displaying tags for", meta)
						err := spacego.GetTag(tags, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, tag *spacego.Tag) error {
							for _, reference := range entry.Record.Reference {
								if bytes.Equal(recordHash, reference.RecordHash) {
									log.Println("\t", tag.Value)
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
				if !success {
					shares, err := bcgo.OpenAndSyncChannel(spacego.SPACE_PREFIX_SHARE + node.Alias)
					if err != nil {
						log.Println(err)
						return
					}
					err = spacego.GetShare(shares, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, share *spacego.Share) error {
						for _, reference := range entry.Record.Reference {
							if bytes.Equal(recordHash, reference.RecordHash) {
								err = spacego.GetSharedMeta(entry.Record.Creator, recordHash, share.MetaKey, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
									if len(os.Args) > 3 {
										ts := os.Args[3:]
										log.Println("Tagging", meta.Name, "with", ts)
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
												RecordHash:  recordHash,
											}}
											reference, err := node.Mine(tags, acl, references, data)
											if err != nil {
												return err
											}
											log.Println("Tagged", os.Args[2], t, base64.RawURLEncoding.EncodeToString(reference.RecordHash))
										}
									} else {
										log.Println("Displaying tags for", meta)
										err := spacego.GetTag(tags, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, key []byte, tag *spacego.Tag) error {
											for _, reference := range entry.Record.Reference {
												if bytes.Equal(recordHash, reference.RecordHash) {
													log.Println("\t", tag.Value)
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
									return err
								}
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
				} else {
					log.Println("Reading from stdin, use CTRL-D to terminate")
				}

				// TODO compress data

				node, err := bcgo.GetNode()
				if err != nil {
					log.Println(err)
					return
				}

				files, err := bcgo.OpenAndSyncChannel(spacego.SPACE_PREFIX_FILE + node.Alias)
				if err != nil {
					log.Println(err)
					return
				}

				metas, err := bcgo.OpenAndSyncChannel(spacego.SPACE_PREFIX_META + node.Alias)
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
					index = index + 1
					reference, err := node.MineRecord(files, record)
					if err != nil {
						return err
					}
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
				log.Println("Cannot handle", os.Args[1])
			}
		}
	} else {
		log.Println("Space Usage:")
		log.Println("\tspace - display usage")
		log.Println("\tspace [name] [type] - read stdin and mine a new record in blockchain")
		log.Println("\tspace [name] [type] [file] - read file and mine a new record in blockchain")
		log.Println("\tspace get [hash] - write file with given hash to stdout")
		log.Println("\tspace get [hash] [file] - write file with given hash to file")
		log.Println("\tspace init - initializes environment, generates key pair, and registers alias")
		log.Println("\tspace list - displays all files created by, or shared with, this key")
		log.Println("\tspace show [hash] - display metadata of file with given hash")
		log.Println("\tspace showall [type] - display metadata of all files with given MIME type")

		log.Println("\tspace search [tag]... - search files for given tags")
		log.Println("\tspace tag [hash] [tag]... - tags file with given hash with given tags")
		log.Println("\tspace share [hash] [alias]... - shares file with given hash with given aliases")

		log.Println("\tspace customer - display Stripe customer information")
		log.Println("\tspace subscription - display String subscription information")
		log.Println("\tspace post [name] [type] - read stdin and posts a new record to Aletheia Ware's Remote Mining Service for mining into blockchain")
		log.Println("\tspace post [name] [type] [file] - read file and posts a new record to Aletheia Ware's Remote Mining Service for mining into blockchain")

		log.Println("BC Usage")
		log.Println("\tbc sync [channel] - synchronizes cache for given channel")
		log.Println("\tbc head [channel] - display head of given channel")
		log.Println("\tbc block [channel] [hash] - display block with given hash on given channel")
		log.Println("\tbc record [channel] [hash] - display record with given hash on given channel")

		log.Println("\tbc alias [alias] - display public key for alias")
		log.Println("\tbc node - display registered alias and public key")

		log.Println("\tbc import-keys [alias] [access-code] - imports the alias and keypair from BC server")
		log.Println("\tbc export-keys [alias] - generates a new access code and exports the alias and keypair to BC server")

		log.Println("\tbc cache - display location of cache")
		log.Println("\tbc keystore - display location of keystore")
		log.Println("\tbc peers - display list of peers")
		log.Println("\tbc add-peer [host] - adds the given host to the list of peers")

		log.Println("\tbc random - generate a random number")
	}
}

func ShowFileShort(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
	hash := base64.RawURLEncoding.EncodeToString(entry.RecordHash)
	timestamp := bcgo.TimestampToString(entry.Record.Timestamp)
	size := bcgo.SizeToString(meta.Size)
	log.Println(hash, timestamp, meta.Name, meta.Type, size)
	return nil
}

func ShowFileLong(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
	hash := base64.RawURLEncoding.EncodeToString(entry.RecordHash)
	timestamp := bcgo.TimestampToString(entry.Record.Timestamp)
	size := bcgo.SizeToString(meta.Size)
	log.Println("Hash:", hash)
	log.Println("Timestamp:", timestamp)
	log.Println("Name:", meta.Name)
	log.Println("Type:", meta.Type)
	log.Println("Size:", size)
	log.Println("Chunks:", len(entry.Record.Reference))
	for index, reference := range entry.Record.Reference {
		hash := base64.RawURLEncoding.EncodeToString(reference.RecordHash)
		log.Println("\t", index, hash)
	}
	return nil
}
