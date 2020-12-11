/*
 * Copyright 2020 Aletheia Ware LLC
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
	"flag"
	"fmt"
	"github.com/AletheiaWareLLC/bcclientgo"
	"github.com/AletheiaWareLLC/bcgo"
	"github.com/AletheiaWareLLC/financego"
	"github.com/AletheiaWareLLC/spaceclientgo"
	"github.com/AletheiaWareLLC/spacego"
	"io"
	"log"
	"os"
	"path/filepath"
)

var peer = flag.String("peer", "", "Space peer")

func main() {
	// Parse command line flags
	flag.Parse()

	// Set log flags
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	client := spaceclientgo.NewSpaceClient(bcgo.SplitRemoveEmpty(*peer, ",")...)

	args := flag.Args()

	if len(args) > 0 {
		switch args[0] {
		case "init":
			PrintLegalese(os.Stdout)
			node, err := client.Init(&bcgo.PrintingMiningListener{Output: os.Stdout})
			if err != nil {
				log.Println(err)
				return
			}
			log.Println("Initialized")
			if err := bcclientgo.PrintNode(os.Stdout, node); err != nil {
				log.Println(err)
				return
			}
		case "add":
			if len(args) > 2 {
				node, err := client.GetNode()
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
				reference, err := client.Add(node, &bcgo.PrintingMiningListener{Output: os.Stdout}, name, mime, reader)
				if err != nil {
					log.Println(err)
					return
				}
				log.Println("Mined metadata", base64.RawURLEncoding.EncodeToString(reference.RecordHash))
			} else {
				log.Println("add <name> <mime> <file>")
				log.Println("add <name> <mime> (data read from stdin)")
			}
		case "list":
			var mimes []string
			if len(args) > 1 {
				mimes = args[1:]
			}
			count := 0
			callback := func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
				success := len(mimes) == 0
				for _, m := range mimes {
					if meta.Type == m {
						success = true
					}
				}
				if !success {
					return nil
				}
				count += 1
				return PrintMeta(os.Stdout, entry, meta)
			}

			node, err := client.GetNode()
			if err != nil {
				log.Println(err)
				return
			}

			log.Println("Files:")
			if err := client.List(node, callback); err != nil {
				log.Println(err)
				return
			}
			log.Println(count, "files")
		case "show":
			if len(args) > 1 {
				node, err := client.GetNode()
				if err != nil {
					log.Println(err)
					return
				}
				recordHash, err := base64.RawURLEncoding.DecodeString(args[1])
				if err != nil {
					log.Println(err)
					return
				}
				if err := client.GetMeta(node, recordHash, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
					return PrintMeta(os.Stdout, entry, meta)
				}); err != nil {
					log.Println(err)
					return
				}
			} else {
				log.Println("show <file-hash>")
			}
		case "get":
			if len(args) > 1 {
				node, err := client.GetNode()
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
					log.Println("Writing to " + args[2])
					writer, err = os.OpenFile(args[2], os.O_CREATE|os.O_WRONLY, os.ModePerm)
					if err != nil {
						log.Println(err)
						return
					}
				}
				count, err := client.ReadFile(node, recordHash, writer)
				if err != nil {
					log.Println(err)
					return
				}

				log.Println("Wrote", bcgo.BinarySizeToString(uint64(count)))
			} else {
				log.Println("get <hash> <file>")
				log.Println("get <hash> (write to stdout)")
			}
		case "get-all":
			if len(args) > 1 {
				node, err := client.GetNode()
				if err != nil {
					log.Println(err)
					return
				}
				if err := client.List(node, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
					hash := base64.RawURLEncoding.EncodeToString(entry.RecordHash)
					dir := filepath.Join(args[1], hash)
					if err := os.MkdirAll(dir, os.ModePerm); err != nil {
						return err
					}
					ext, err := getExtension(meta.Type)
					if err != nil {
						return err
					}
					path := filepath.Join(dir, meta.Name+"."+ext)
					if _, err := os.Stat(path); os.IsNotExist(err) {
						log.Println("Writing to " + path)
						writer, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, os.ModePerm)
						if err != nil {
							return err
						}
						count, err := client.ReadFile(node, entry.RecordHash, writer)
						if err != nil {
							return err
						}
						log.Println("Wrote", bcgo.BinarySizeToString(uint64(count)))
					}
					return nil
				}); err != nil {
					log.Println(err)
					return
				}
			} else {
				log.Println("get-all <directory>")
			}
		case "search":
			// search metas by tag
			if len(args) > 1 {
				ts := args[1:]
				log.Println("Searching Files for", ts)
				node, err := client.GetNode()
				if err != nil {
					log.Println(err)
					return
				}
				log.Println("Files:")
				count := 0
				if client.Search(node, ts, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
					count += 1
					return PrintMeta(os.Stdout, entry, meta)
				}); err != nil {
					log.Println(err)
					return
				}
				log.Println(count, "files")
			} else {
				log.Println("search <tag>... (search files by tags)")
			}
		case "tag":
			if len(args) > 1 {
				node, err := client.GetNode()
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

					references, err := client.AddTag(node, &bcgo.PrintingMiningListener{Output: os.Stdout}, recordHash, tags)
					if err != nil {
						log.Println(err)
						return
					}

					log.Println("Tagged", args[1], references)
				} else {
					if err := client.GetTag(node, recordHash, func(entry *bcgo.BlockEntry, tag *spacego.Tag) {
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
		case "registration":
			merchant := ""
			if len(args) > 1 {
				merchant = args[1]
			}
			count := 0
			if err := client.GetRegistration(merchant, func(r *financego.Registration) error {
				log.Println(r)
				count++
				return nil
			}); err != nil {
				log.Println(err)
				return
			}
			log.Println(count, "results")
		case "subscription":
			merchant := ""
			if len(args) > 1 {
				merchant = args[1]
			}
			count := 0
			if err := client.GetSubscription(merchant, func(s *financego.Subscription) error {
				log.Println(s)
				count++
				return nil
			}); err != nil {
				log.Println(err)
				return
			}
			log.Println(count, "results")
		case "registrars":
			node, err := client.GetNode()
			if err != nil {
				log.Println(err)
				return
			}
			count := 0
			if err := client.GetRegistrarsForNode(node, func(a *spacego.Registrar, r *financego.Registration, s *financego.Subscription) error {
				log.Println(a, r, s)
				count++
				return nil
			}); err != nil {
				log.Println(err)
				return
			}
			log.Println(count, "results")
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
	fmt.Fprintln(output, "\tspace add [name] [type] - read stdin and mine a new record into blockchain")
	fmt.Fprintln(output, "\tspace add [name] [type] [file] - read file and mine a new record into blockchain")
	// TODO fmt.Fprintln(output, "\tspace add-directory [directory] - read all files in directory and mine new records into blockchain")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "\tspace list - prints all files created by this key")
	fmt.Fprintln(output, "\tspace list [type] - display metadata of all files with given MIME type")
	fmt.Fprintln(output, "\tspace show [hash] - display metadata of file with given hash")
	// TODO fmt.Fprintln(output, "\tspace show-keys [hash] - display keys of file with given hash")
	fmt.Fprintln(output, "\tspace get [hash] - write file with given hash to stdout")
	fmt.Fprintln(output, "\tspace get [hash] [file] - write file with given hash to file")
	fmt.Fprintln(output, "\tspace get-all [directory] - write all files to given directory")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "\tspace tag [hash] [tag...] - tags file with given hash with given tags")
	fmt.Fprintln(output, "\tspace search [tag...] - search files for given tags")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "\tspace registration [merchant] - display registration information between this alias and the given merchant")
	fmt.Fprintln(output, "\tspace subscription [merchant] - display subscription information between this alias and the given merchant")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "\tspace registrars - display registration and subscription information of this alias' registrars")
}

func PrintLegalese(output io.Writer) {
	fmt.Fprintln(output, "S P A C E Legalese:")
	fmt.Fprintln(output, "S P A C E is made available by Aletheia Ware LLC [https://aletheiaware.com] under the Terms of Service [https://aletheiaware.com/terms-of-service.html] and Privacy Policy [https://aletheiaware.com/privacy-policy.html].")
	fmt.Fprintln(output, "This beta version of S P A C E is made available under the Beta Test Agreement [https://aletheiaware.com/space-beta-test-agreement.html].")
	fmt.Fprintln(output, "By continuing to use this software you agree to the Terms of Service, Privacy Policy, and Beta Test Agreement.")
}

func PrintMeta(output io.Writer, entry *bcgo.BlockEntry, meta *spacego.Meta) error {
	hash := base64.RawURLEncoding.EncodeToString(entry.RecordHash)
	timestamp := bcgo.TimestampToString(entry.Record.Timestamp)
	fmt.Fprintf(output, "%s %s %s %s\n", hash, timestamp, meta.Name, meta.Type)
	return nil
}

func getExtension(mime string) (string, error) {
	switch mime {
	case spacego.MIME_TYPE_IMAGE_JPG, spacego.MIME_TYPE_IMAGE_JPEG:
		return "jpg", nil
	case spacego.MIME_TYPE_TEXT_PLAIN:
		return "txt", nil
	case spacego.MIME_TYPE_VIDEO_MPEG:
		return "mpg", nil
	}
	return "", fmt.Errorf("Unrecognized Mime: %s", mime)
}
