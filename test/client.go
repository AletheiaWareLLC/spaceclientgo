/*
 * Copyright 2021 Aletheia Ware LLC
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

package test

import (
	"aletheiaware.com/bcclientgo/test"
	"aletheiaware.com/bcgo"
	"aletheiaware.com/financego"
	"aletheiaware.com/spacego"
	"context"
	"io"
	"testing"
)

func NewMockSpaceClient(t *testing.T) *MockSpaceClient {
	t.Helper()
	return &MockSpaceClient{
		MockBCClient: test.MockBCClient{},
	}
}

type MockSpaceClient struct {
	test.MockBCClient
	MockContext                     context.Context
	MockName, MockMime              string
	MockReference                   *bcgo.Reference
	MockReferences                  []*bcgo.Reference
	MockDeltaChannel                bcgo.Channel
	MockDeltas                      []*spacego.Delta
	MockHash                        []byte
	MockMetaFilter                  spacego.MetaFilter
	MockMetaCallback                spacego.MetaCallback
	MockMetaCallbackResults         []*MockMetaCallbackResult
	MockWriteCloser                 io.WriteCloser
	MockTagFilter                   spacego.TagFilter
	MockTags                        []string
	MockMerchant                    string
	MockRegistrationCallback        financego.RegistrationCallback
	MockRegistrationCallbackResults []*MockRegistrationCallbackResult
	MockSubscriptionCallback        financego.SubscriptionCallback
	MockSubscriptionCallbackResults []*MockSubscriptionCallbackResult

	MockAddError, MockAppendError                error
	MockMetaError, MockAllMetasError             error
	MockReadError, MockWriteError                error
	MockAddTagError, MockAllTagsError            error
	MockSearchMetaError, MockSearchTagError      error
	MockRegistrationError, MockSubscriptionError error
}

func (c *MockSpaceClient) Add(node bcgo.Node, listener bcgo.MiningListener, name, mime string, reader io.Reader) (*bcgo.Reference, error) {
	c.MockNode = node
	c.MockListener = listener
	c.MockName = name
	c.MockMime = mime
	c.MockReader = reader
	return c.MockReference, c.MockAddError
}

func (c *MockSpaceClient) Append(node bcgo.Node, listener bcgo.MiningListener, channel bcgo.Channel, deltas ...*spacego.Delta) error {
	c.MockNode = node
	c.MockListener = listener
	c.MockDeltaChannel = channel
	c.MockDeltas = deltas
	return c.MockAppendError
}

func (c *MockSpaceClient) MetaForHash(node bcgo.Node, hash []byte, callback spacego.MetaCallback) error {
	c.MockNode = node
	c.MockHash = hash
	c.MockMetaCallback = callback
	for _, r := range c.MockMetaCallbackResults {
		callback(r.Entry, r.Meta)
	}
	return c.MockMetaError
}

func (c *MockSpaceClient) AllMetas(node bcgo.Node, callback spacego.MetaCallback) error {
	c.MockNode = node
	c.MockMetaCallback = callback
	for _, r := range c.MockMetaCallbackResults {
		callback(r.Entry, r.Meta)
	}
	return c.MockAllMetasError
}

func (c *MockSpaceClient) ReadFile(node bcgo.Node, hash []byte) (io.Reader, error) {
	c.MockNode = node
	c.MockHash = hash
	return c.MockReader, c.MockReadError
}

func (c *MockSpaceClient) WriteFile(node bcgo.Node, listener bcgo.MiningListener, hash []byte) (io.WriteCloser, error) {
	c.MockNode = node
	c.MockListener = listener
	c.MockHash = hash
	return c.MockWriteCloser, c.MockWriteError
}

func (c *MockSpaceClient) WatchFile(ctx context.Context, node bcgo.Node, hash []byte, callback func()) {
	c.MockContext = ctx
	c.MockNode = node
	c.MockHash = hash
}

func (c *MockSpaceClient) AddTag(node bcgo.Node, listener bcgo.MiningListener, hash []byte, tags []string) ([]*bcgo.Reference, error) {
	c.MockNode = node
	c.MockListener = listener
	c.MockHash = hash
	c.MockTags = tags
	return c.MockReferences, c.MockAddTagError
}

func (c *MockSpaceClient) AllTagsForHash(node bcgo.Node, hash []byte, callback spacego.TagCallback) error {
	c.MockNode = node
	c.MockHash = hash
	return c.MockAllTagsError
}

func (c *MockSpaceClient) SearchMeta(node bcgo.Node, filter spacego.MetaFilter, callback spacego.MetaCallback) error {
	c.MockNode = node
	c.MockMetaFilter = filter
	c.MockMetaCallback = callback
	for _, r := range c.MockMetaCallbackResults {
		callback(r.Entry, r.Meta)
	}
	return c.MockSearchMetaError
}

func (c *MockSpaceClient) SearchTag(node bcgo.Node, filter spacego.TagFilter, callback spacego.MetaCallback) error {
	c.MockNode = node
	c.MockTagFilter = filter
	c.MockMetaCallback = callback
	for _, r := range c.MockMetaCallbackResults {
		callback(r.Entry, r.Meta)
	}
	return c.MockSearchTagError
}

func (c *MockSpaceClient) Registration(merchant string, callback financego.RegistrationCallback) error {
	c.MockMerchant = merchant
	c.MockRegistrationCallback = callback
	for _, r := range c.MockRegistrationCallbackResults {
		callback(r.Entry, r.Registration)
	}
	return c.MockRegistrationError
}

func (c *MockSpaceClient) Subscription(merchant string, callback financego.SubscriptionCallback) error {
	c.MockMerchant = merchant
	c.MockSubscriptionCallback = callback
	for _, r := range c.MockSubscriptionCallbackResults {
		callback(r.Entry, r.Subscription)
	}
	return c.MockSubscriptionError
}

type MockMetaCallbackResult struct {
	Entry *bcgo.BlockEntry
	Meta  *spacego.Meta
}

type MockRegistrationCallbackResult struct {
	Entry        *bcgo.BlockEntry
	Registration *financego.Registration
}

type MockSubscriptionCallbackResult struct {
	Entry        *bcgo.BlockEntry
	Subscription *financego.Subscription
}
