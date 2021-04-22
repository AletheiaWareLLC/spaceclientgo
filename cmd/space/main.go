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
	"aletheiaware.com/aliasgo"
	"aletheiaware.com/bcclientgo"
	"aletheiaware.com/bcgo"
	"aletheiaware.com/financego"
	"aletheiaware.com/spaceclientgo"
	"aletheiaware.com/spacego"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
			root, err := client.Root()
			if err != nil {
				log.Println(err)
				return
			}
			// Add Space hosts to peers
			for _, host := range spacego.SpaceHosts() {
				if err := bcgo.AddPeer(root, host); err != nil {
					log.Println(err)
					return
				}
			}
			// Add BC host to peers
			if err := bcgo.AddPeer(root, bcgo.BCHost()); err != nil {
				log.Println(err)
				return
			}
			node, err := client.Node()
			if err != nil {
				log.Println(err)
				return
			}
			if err := aliasgo.Register(node, &bcgo.PrintingMiningListener{Output: os.Stdout}); err != nil {
				log.Println(err)
				return
			}
			if err != nil {
				log.Println(err)
				return
			}
			log.Println("Initialized")
			if err := bcclientgo.PrintIdentity(os.Stdout, node.Account()); err != nil {
				log.Println(err)
				return
			}
		case "add":
			if len(args) > 2 {
				node, err := client.Node()
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

			node, err := client.Node()
			if err != nil {
				log.Println(err)
				return
			}

			log.Println("Files:")
			if err := client.AllMetas(node, callback); err != nil {
				log.Println(err)
				return
			}
			log.Println(count, "files")
		case "show":
			if len(args) > 1 {
				node, err := client.Node()
				if err != nil {
					log.Println(err)
					return
				}
				recordHash, err := base64.RawURLEncoding.DecodeString(args[1])
				if err != nil {
					log.Println(err)
					return
				}
				if err := client.MetaForHash(node, recordHash, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
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
				node, err := client.Node()
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
				reader, err := client.ReadFile(node, recordHash)
				if err != nil {
					log.Println(err)
					return
				}
				count, err := io.Copy(writer, reader)
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
				node, err := client.Node()
				if err != nil {
					log.Println(err)
					return
				}
				if err := client.AllMetas(node, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
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
						reader, err := client.ReadFile(node, entry.RecordHash)
						if err != nil {
							return err
						}
						count, err := io.Copy(writer, reader)
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
		case "set":
			if len(args) > 1 {
				node, err := client.Node()
				if err != nil {
					log.Println(err)
					return
				}
				recordHash, err := base64.RawURLEncoding.DecodeString(args[1])
				if err != nil {
					log.Println(err)
					return
				}
				reader := os.Stdin
				if len(args) > 2 {
					log.Println("Reading from " + args[2])
					reader, err = os.Open(args[2])
					if err != nil {
						log.Println(err)
						return
					}
				}
				writer, err := client.WriteFile(node, &bcgo.PrintingMiningListener{Output: os.Stdout}, recordHash)
				if err != nil {
					log.Println(err)
					return
				}
				count, err := io.Copy(writer, reader)
				if err != nil {
					log.Println(err)
					return
				}
				if err := writer.Close(); err != nil {
					log.Println(err)
					return
				}
				log.Println("Wrote", bcgo.BinarySizeToString(uint64(count)))
			} else {
				log.Println("set <hash> <file>")
				log.Println("set <hash> (read from stdin)")
			}
		case "search":
			// search files by name, type, and/or tag
			if len(args) > 1 {
				var names, types, tags []string
				for _, a := range args[1:] {
					switch {
					case strings.HasPrefix(a, "tag:"):
						tags = append(tags, strings.TrimPrefix(a, "tag:"))
					case strings.HasPrefix(a, "type:"):
						types = append(types, strings.TrimPrefix(a, "type:"))
					case strings.HasPrefix(a, "name:"):
						fallthrough
					default:
						names = append(names, strings.TrimPrefix(a, "name:"))
					}
				}
				node, err := client.Node()
				if err != nil {
					log.Println(err)
					return
				}
				log.Println("Files:")
				var hashes []string
				counts := make(map[string]int)
				entries := make(map[string]*bcgo.BlockEntry)
				metas := make(map[string]*spacego.Meta)
				callback := func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
					hash := base64.RawURLEncoding.EncodeToString(entry.RecordHash)
					c, ok := counts[hash]
					if ok {
						c = c + 1
					} else {
						c = 1
						hashes = append(hashes, hash)
						entries[hash] = entry
						metas[hash] = meta
					}
					counts[hash] = c
					return nil
				}
				// Search by name
				if len(names) > 0 {
					if client.SearchMeta(node, spacego.NewNameFilter(names...), callback); err != nil {
						log.Println(err)
						return
					}
				}
				// Search by type
				if len(types) > 0 {
					if client.SearchMeta(node, spacego.NewTypeFilter(types...), callback); err != nil {
						log.Println(err)
						return
					}
				}
				// Search by tag
				if len(tags) > 0 {
					if client.SearchTag(node, spacego.NewTagFilter(tags...), callback); err != nil {
						log.Println(err)
						return
					}
				}
				// Sort by timestamp
				sort.Slice(hashes, func(i, j int) bool {
					return entries[hashes[i]].Record.Timestamp < entries[hashes[j]].Record.Timestamp
				})
				// Sort by count
				sort.Slice(hashes, func(i, j int) bool {
					return counts[hashes[i]] < counts[hashes[j]]
				})
				for _, h := range hashes {
					PrintMeta(os.Stdout, entries[h], metas[h])
				}
				log.Println(len(hashes), "files")
			} else {
				log.Println("search <name> (search files by name)")
				log.Println("search name:<name> (search files by name)")
				log.Println("search type:<type> (search files by type)")
				log.Println("search tag:<tag> (search files by tag)")
			}
		case "tag":
			if len(args) > 1 {
				node, err := client.Node()
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
					if err := client.AllTagsForHash(node, recordHash, func(entry *bcgo.BlockEntry, tag *spacego.Tag) error {
						log.Println(tag.Value)
						return nil
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
			if err := client.Registration(merchant, func(e *bcgo.BlockEntry, r *financego.Registration) error {
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
			if err := client.Subscription(merchant, func(e *bcgo.BlockEntry, s *financego.Subscription) error {
				log.Println(s)
				count++
				return nil
			}); err != nil {
				log.Println(err)
				return
			}
			log.Println(count, "results")
		case "registrars":
			node, err := client.Node()
			if err != nil {
				log.Println(err)
				return
			}
			count := 0
			if err := spacego.AllRegistrarsForNode(node, func(a *spacego.Registrar, r *financego.Registration, s *financego.Subscription) error {
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
	fmt.Fprintln(output, "\tspace set [hash] - write stdin to file with given hash")
	fmt.Fprintln(output, "\tspace set [hash] [file] - write file to file with given hash")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "\tspace tag [hash] [tag...] - tags file with given hash with given tags")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "\tspace search name:[name] - search files for given name")
	fmt.Fprintln(output, "\tspace search type:[type] - search files for given type")
	fmt.Fprintln(output, "\tspace search tag:[tag] - search files for given tag")
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
