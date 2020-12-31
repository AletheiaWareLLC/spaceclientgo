/*
 * Copyright 2019-2020 Aletheia Ware LLC
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

package spaceclientgo_test

import (
	"aletheiaware.com/bcgo"
	"aletheiaware.com/spaceclientgo"
	"aletheiaware.com/spacego"
	"aletheiaware.com/testinggo"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"log"
	"strings"
	"testing"
)

func assertFile(t *testing.T, c *spaceclientgo.SpaceClient, n *bcgo.Node, metaId []byte, length int, content string) {
	t.Helper()
	var buf bytes.Buffer
	count, err := c.ReadFile(n, metaId, &buf)
	testinggo.AssertNoError(t, err)
	log.Println("File", count, buf.String())
	if count != length {
		t.Fatalf("Incorrect length; expected '%d', got '%d'", length, count)
	}
	actual := buf.String()
	if actual != content {
		t.Fatalf("Incorrect content; expected '%s', got '%s", content, actual)
	}
}

func assertMeta(t *testing.T, c *spaceclientgo.SpaceClient, n *bcgo.Node, name, mime string) {
	t.Helper()
	var metas []*spacego.Meta
	testinggo.AssertNoError(t, c.List(n, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
		metas = append(metas, meta)
		return nil
	}))
	if len(metas) != 1 {
		t.Fatalf("Expected a meta")
	}
	meta := metas[0]
	if name != meta.Name {
		t.Fatalf("Incorrect name; expected '%s', got '%s'", name, meta.Name)
	}
	if mime != meta.Type {
		t.Fatalf("Incorrect mime; expected '%s', got '%s'", mime, meta.Type)
	}
}

func makeNode(t *testing.T, a string, key *rsa.PrivateKey, cache bcgo.Cache, network bcgo.Network) *bcgo.Node {
	t.Helper()
	return &bcgo.Node{
		Alias:    a,
		Key:      key,
		Cache:    cache,
		Network:  network,
		Channels: make(map[string]*bcgo.Channel),
	}
}

func TestClientInit(t *testing.T) {
	// TODO set ROOT_DIRECTORY, ALIAS env
	/*
		t.Run("Success", func(t *testing.T) {
		   root := testinggo.MakeEnvTempDir(t, "ROOT_DIRECTORY", "root")
		   defer testinggo.UnmakeEnvTempDir(t, "ROOT_DIRECTORY", root)
		   client := &main.Client{
		       Root: root,
		   }
		   node, err := client.Init()
		   testinggo.AssertNoError(t, err)
		})
		t.Run("AliasAlreadyRegistered", func(t *testing.T) {
		})
	*/
}

func TestClient_Add_and_ReadFile(t *testing.T) {
	alias := "Tester"
	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		t.Error("Could not generate key:", err)
	}
	cache := bcgo.NewMemoryCache(10)
	node := makeNode(t, alias, key, cache, nil)
	client := &spaceclientgo.SpaceClient{}
	name := "test"
	mime := "text/plain"
	ref, err := client.Add(node, nil, name, mime, strings.NewReader("testing"))
	testinggo.AssertNoError(t, err)

	assertMeta(t, client, node, name, mime)
	assertFile(t, client, node, ref.RecordHash, 7, "testing")
}

func TestClient_Append_and_ReadFile(t *testing.T) {
	alias := "Tester"
	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		t.Error("Could not generate key:", err)
	}
	cache := bcgo.NewMemoryCache(10)
	node := makeNode(t, alias, key, cache, nil)
	client := &spaceclientgo.SpaceClient{}
	name := "test"
	mime := "text/plain"
	ref, err := client.Add(node, nil, name, mime, strings.NewReader("testing"))
	testinggo.AssertNoError(t, err)

	assertMeta(t, client, node, name, mime)
	assertFile(t, client, node, ref.RecordHash, 7, "testing")

	metaId := base64.RawURLEncoding.EncodeToString(ref.RecordHash)
	deltas := spacego.OpenDeltaChannel(metaId)
	testinggo.AssertNoError(t, deltas.LoadCachedHead(node.Cache))

	acl := map[string]*rsa.PublicKey{
		alias: &key.PublicKey,
	}

	testinggo.AssertNoError(t, client.Append(node, nil, deltas, acl, &spacego.Delta{
		Offset: 4,
		Remove: 3,
		Add:    []byte("foobar"),
	}))
	assertFile(t, client, node, ref.RecordHash, 10, "testfoobar")

	testinggo.AssertNoError(t, client.Append(node, nil, deltas, acl, &spacego.Delta{
		Remove: 7,
	}))
	assertFile(t, client, node, ref.RecordHash, 3, "bar")
}

func TestClientList(t *testing.T) {
	alias := "Tester"
	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		t.Error("Could not generate key:", err)
	}
	cache := bcgo.NewMemoryCache(10)
	node := makeNode(t, alias, key, cache, nil)
	client := &spaceclientgo.SpaceClient{}
	name0 := "test0"
	mime0 := "text/plain"
	ref0, err := client.Add(node, nil, name0, mime0, strings.NewReader("testing0"))
	testinggo.AssertNoError(t, err)

	name1 := "test1"
	mime1 := "text/plain"
	ref1, err := client.Add(node, nil, name1, mime1, strings.NewReader("testing1"))
	testinggo.AssertNoError(t, err)

	results := make(map[string]*spacego.Meta)
	testinggo.AssertNoError(t, client.List(node, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
		results[base64.RawURLEncoding.EncodeToString(entry.RecordHash)] = meta
		return nil
	}))
	if len(results) != 2 {
		t.Fatalf("Expected 2 results")
	}
	meta0 := results[base64.RawURLEncoding.EncodeToString(ref0.RecordHash)]
	if name0 != meta0.Name {
		t.Fatalf("Incorrect name; expected '%s', got '%s'", name0, meta0.Name)
	}
	if mime0 != meta0.Type {
		t.Fatalf("Incorrect mime; expected '%s', got '%s'", mime0, meta0.Type)
	}
	meta1 := results[base64.RawURLEncoding.EncodeToString(ref1.RecordHash)]
	if name1 != meta1.Name {
		t.Fatalf("Incorrect name; expected '%s', got '%s'", name1, meta1.Name)
	}
	if mime1 != meta1.Type {
		t.Fatalf("Incorrect mime; expected '%s', got '%s'", mime1, meta1.Type)
	}
}

func TestClientGetMeta(t *testing.T) {
	alias := "Tester"
	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		t.Error("Could not generate key:", err)
	}
	cache := bcgo.NewMemoryCache(10)
	node := makeNode(t, alias, key, cache, nil)
	client := &spaceclientgo.SpaceClient{}
	name := "test"
	mime := "text/plain"
	ref, err := client.Add(node, nil, name, mime, strings.NewReader("testing"))
	testinggo.AssertNoError(t, err)
	var metas []*spacego.Meta
	testinggo.AssertNoError(t, client.GetMeta(node, ref.RecordHash, func(entry *bcgo.BlockEntry, meta *spacego.Meta) error {
		metas = append(metas, meta)
		return nil
	}))
	if len(metas) != 1 {
		t.Fatalf("Expected a meta")
	}
	meta := metas[0]
	if name != meta.Name {
		t.Fatalf("Incorrect name; expected '%s', got '%s'", name, meta.Name)
	}
	if mime != meta.Type {
		t.Fatalf("Incorrect mime; expected '%s', got '%s'", mime, meta.Type)
	}
}

func TestClientSearch(t *testing.T) {
	// TODO
}

func TestClientAddTag(t *testing.T) {
	// TODO
}

func TestClientGetTag(t *testing.T) {
	// TODO
}

func TestClientGetRegistration(t *testing.T) {
	// TODO
}

func TestClientGetSubscription(t *testing.T) {
	// TODO
}

func TestClientGetRegistrarsForNode(t *testing.T) {
	// TODO
}
