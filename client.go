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
	"github.com/AletheiaWareLLC/aliasgo"
	"github.com/AletheiaWareLLC/bcgo"
	"github.com/AletheiaWareLLC/financego"
	"github.com/AletheiaWareLLC/spacego"
	"github.com/golang/protobuf/proto"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
)

func main() {
	if len(os.Args) > 1 {
		// Handle Arguments
		switch os.Args[1] {
		case "alias":
			node, err := bcgo.GetNode()
			if err != nil {
				log.Println(err)
				return
			}
			publicKey, err := bcgo.RSAPublicKeyToBytes(&node.Key.PublicKey)
			if err != nil {
				log.Println(err)
				return
			}
			// Open Alias Channel
			aliases, err := aliasgo.OpenAliasChannel()
			if err != nil {
				log.Println(err)
				return
			}
			// Sync channel
			if err := aliases.Sync(); err != nil {
				log.Println(err)
				return
			}
			alias, err := aliasgo.GetAlias(aliases, &node.Key.PublicKey)
			if err != nil {
				log.Println(err)
				a := &aliasgo.Alias{
					Alias:        alias,
					PublicKey:    publicKey,
					PublicFormat: bcgo.PublicKeyFormat_PKIX,
				}
				data, err := proto.Marshal(a)
				if err != nil {
					log.Println(err)
					return
				}

				signatureAlgorithm := bcgo.SignatureAlgorithm_SHA512WITHRSA_PSS

				signature, err := bcgo.CreateSignature(node.Key, bcgo.Hash(data), signatureAlgorithm)
				if err != nil {
					log.Println(err)
					return
				}

				response, err := http.PostForm(spacego.SPACE_WEBSITE+"/alias", url.Values{
					"alias":              {alias},
					"publicKey":          {base64.RawURLEncoding.EncodeToString(publicKey)},
					"publicKeyFormat":    {"PKIX"},
					"signature":          {base64.RawURLEncoding.EncodeToString(signature)},
					"signatureAlgorithm": {signatureAlgorithm.String()},
				})
				if err != nil {
					log.Println(err)
					return
				}
				log.Println(response)
				if err := aliases.Sync(); err != nil {
					log.Println(err)
					return
				}
				alias, err = aliasgo.GetAlias(aliases, &node.Key.PublicKey)
				if err != nil {
					log.Println(err)
					return
				}
			}
			log.Println("Registered as", alias)
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
				customer, err := spacego.GetCustomer(node)
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
			customer, err := spacego.GetCustomer(node)
			if err != nil {
				publicKey, err := bcgo.RSAPublicKeyToBase64(&node.Key.PublicKey)
				if err != nil {
					log.Println(err)
					return
				}
				log.Println(err)
				log.Println("To subscribe for remote mining, visit", spacego.SPACE_WEBSITE+"/customer?alias="+node.Alias, " and")
				log.Println("enter your alias, email, payment info, and public key:\n", publicKey)
			} else {
				log.Println(customer)
			}
		case "subscription":
			node, err := bcgo.GetNode()
			if err != nil {
				log.Println(err)
				return
			}
			subscription, err := spacego.GetSubscription(node)
			if err != nil {
				publicKey, err := bcgo.RSAPublicKeyToBase64(&node.Key.PublicKey)
				if err != nil {
					log.Println(err)
					return
				}
				log.Println(err)
				log.Println("To subscribe for remote mining, visit", spacego.SPACE_WEBSITE+"/subscription?alias="+node.Alias, " and")
				log.Println("enter your alias, email, payment info, and public key:\n", publicKey)
			} else {
				log.Println(subscription)
			}
		case "status":
			response, err := http.Get(spacego.SPACE_WEBSITE + "/status")
			if err != nil {
				log.Println(err)
				return
			}
			log.Println(response)
		case "stripe-webhook":
			// TODO
		default:
			log.Println("Unable to handle", os.Args)
		}
	} else {
		response, err := http.Get(spacego.SPACE_WEBSITE)
		if err != nil {
			log.Println(err)
			return
		}
		log.Println(response)
	}
}
