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
	"encoding/base64"
	"github.com/AletheiaWareLLC/bcgo"
	"github.com/AletheiaWareLLC/financego"
	"github.com/AletheiaWareLLC/spacego"
	"github.com/golang/protobuf/proto"
	"io/ioutil"
	"log"
	"os"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	if len(os.Args) > 1 {
		// Handle Arguments
		switch os.Args[1] {
		case "list":
			node, err := bcgo.GetNode()
			if err != nil {
				log.Println(err)
				return
			}
			metas, err := spacego.OpenMetaChannel(node.Alias)
			if err != nil {
				log.Println(err)
				return
			}
			// List files owned by key
			if err := spacego.GetMeta(metas, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, meta *spacego.Meta) {
				hash := base64.RawURLEncoding.EncodeToString(entry.RecordHash)
				log.Println("hash:", hash, meta)
			}); err != nil {
				log.Println(err)
				return
			}
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
				metas, err := spacego.OpenMetaChannel(node.Alias)
				if err != nil {
					log.Println(err)
					return
				}
				if err := spacego.GetMeta(metas, node.Alias, node.Key, recordHash, func(entry *bcgo.BlockEntry, meta *spacego.Meta) {
					log.Println(meta)
				}); err != nil {
					log.Println(err)
					return
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
				metas, err := spacego.OpenMetaChannel(node.Alias)
				if err != nil {
					log.Println(err)
					return
				}
				if err := spacego.GetMeta(metas, node.Alias, node.Key, nil, func(entry *bcgo.BlockEntry, meta *spacego.Meta) {
					if meta.Type == os.Args[2] {
						log.Println(meta)
					}
				}); err != nil {
					log.Println(err)
					return
				}
			} else {
				log.Println("showall <mime-type>")
			}
		case "preview":
			// Preview file by given hash
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
				metas, err := spacego.OpenMetaChannel(node.Alias)
				if err != nil {
					log.Println(err)
					return
				}
				if err := spacego.GetMeta(metas, node.Alias, node.Key, recordHash, func(entry *bcgo.BlockEntry, meta *spacego.Meta) {
					references := entry.Record.Reference
					if len(references) > 1 {
						/* TODO
						previewRecordHash := references[1].RecordHash
						p.GetPreview(node.Alias, node.Key, previewRecordHash, func(entry *bcgo.BlockEntry, data []byte) {
							if len(os.Args) > 3 {
								// TODO write data to os.Args[3] file
							} else {
								log.Println(data)
							}
						})
						*/
					} else {
						log.Println("No preview for", os.Args[2])
					}
				}); err != nil {
					log.Println(err)
					return
				}
			} else {
				log.Println("preview <file-hash>")
			}
		case "download":
			// Download file by given hash
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
				metas, err := spacego.OpenMetaChannel(node.Alias)
				if err != nil {
					log.Println(err)
					return
				}
				if err := spacego.GetMeta(metas, node.Alias, node.Key, recordHash, func(entry *bcgo.BlockEntry, meta *spacego.Meta) {
					references := entry.Record.Reference
					fileRecordHash := references[0].RecordHash
					files, err := spacego.OpenFileChannel(node.Alias)
					if err != nil {
						log.Println(err)
						return
					}
					if err := spacego.GetFile(files, node.Alias, node.Key, fileRecordHash, func(entry *bcgo.BlockEntry, data []byte) {
						if len(os.Args) > 3 {
							ioutil.WriteFile(os.Args[3], data, 0600)
							log.Println("downloaded to " + os.Args[3])
						} else {
							os.Stdout.Write(data)
						}
					}); err != nil {
						log.Println(err)
						return
					}
				}); err != nil {
					log.Println(err)
					return
				}
			} else {
				log.Println("download <hash> <file>")
				log.Println("download <hash> (data written to stdout)")
			}
		case "upload":
			if len(os.Args) > 3 {
				name := os.Args[2]
				mime := os.Args[3]
				var data []byte
				var err error
				if len(os.Args) > 4 {
					// Read data from file
					data, err = ioutil.ReadFile(os.Args[4])
				} else {
					// Read data from system in
					data, err = ioutil.ReadAll(os.Stdin)
				}
				if err != nil {
					log.Println(err)
					return
				}

				size := uint64(len(data))

				// TODO compress data

				node, err := bcgo.GetNode()
				if err != nil {
					log.Println(err)
					return
				}

				fileBundle, err := spacego.NewBundle(node, data)
				if err != nil {
					log.Println(err)
					return
				}
				meta := spacego.Meta{
					Name: name,
					Size: size,
					Type: mime,
				}
				data, err = proto.Marshal(&meta)
				if err != nil {
					log.Println(err)
					return
				}
				metaBundle, err := spacego.NewBundle(node, data)
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
					log.Println("No Customer")
					// TODO mine locally
					return
				} else {
					response, err := spacego.Upload(spacego.SPACE_HOST, &spacego.StorageRequest{
						Alias:      node.Alias,
						Processor:  financego.PaymentProcessor_STRIPE,
						CustomerId: customer.CustomerId,
						File:       fileBundle,
						Meta:       metaBundle,
						//Preview: skipped,
					})
					if err != nil {
						log.Println(err)
						return
					}
					log.Println(response)
				}
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
		default:
			if len(os.Args) > 2 {
				name := os.Args[1]
				mime := os.Args[2]
				var data []byte
				var err error
				if len(os.Args) > 3 {
					// Read data from file
					data, err = ioutil.ReadFile(os.Args[3])
				} else {
					// Read data from system in
					data, err = ioutil.ReadAll(os.Stdin)
				}
				if err != nil {
					log.Println(err)
					return
				}

				size := uint64(len(data))

				// TODO compress data

				node, err := bcgo.GetNode()
				if err != nil {
					log.Println(err)
					return
				}

				// Create an array to hold at most two references (file, preview)
				references := make([]*bcgo.Reference, 0, 2)

				fileBundle, err := spacego.NewBundle(node, data)

				files, err := spacego.OpenFileChannel(node.Alias)
				if err != nil {
					log.Println(err)
					return
				}

				fileReference, err := spacego.MineBundle(node, files, node.Alias, &node.Key.PublicKey, fileBundle, nil)
				if err != nil {
					log.Println(err)
					return
				}
				// Add fileReference to list of references
				references = append(references, fileReference)
				log.Println("File:", fileReference)

				// TODO Add previewReference to list of references
				//references = append(references, previewReference)
				//log.Println("Preview:", previewReference)

				meta := spacego.Meta{
					Name: name,
					Size: size,
					Type: mime,
				}
				data, err = proto.Marshal(&meta)
				if err != nil {
					log.Println(err)
					return
				}
				metaBundle, err := spacego.NewBundle(node, data)
				if err != nil {
					log.Println(err)
					return
				}

				metas, err := spacego.OpenMetaChannel(node.Alias)
				if err != nil {
					log.Println(err)
					return
				}
				metaReference, err := spacego.MineBundle(node, metas, node.Alias, &node.Key.PublicKey, metaBundle, references)
				if err != nil {
					log.Println(err)
					return
				}

				log.Println("Meta:", metaReference)
			} else {
				bcgo.GetAndPrintURL(spacego.SPACE_WEBSITE)
			}
		}
	} else {
		bcgo.GetAndPrintURL(spacego.SPACE_WEBSITE)
	}
}
