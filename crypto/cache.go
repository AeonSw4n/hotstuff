package crypto

import (
	"container/list"
	"crypto/sha256"
	"strings"
	"sync"

	"github.com/relab/hotstuff"
	"github.com/relab/hotstuff/consensus"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

type cache struct {
	impl        consensus.CryptoBase
	mut         sync.Mutex
	capacity    int
	entries     map[string]*list.Element
	accessOrder list.List
}

// NewCache returns a new Crypto instance that caches the results of the operations of the given CryptoBase.
// implementation.
func NewCache(impl consensus.CryptoBase, capacity int) consensus.Crypto {
	return New(&cache{
		impl:     impl,
		capacity: capacity,
		entries:  make(map[string]*list.Element, capacity),
	})
}

// InitConsensusModule gives the module a reference to the Modules object.
// It also allows the module to set module options using the OptionsBuilder.
func (cache *cache) InitConsensusModule(mods *consensus.Modules, cfg *consensus.OptionsBuilder) {
	if mod, ok := cache.impl.(consensus.Module); ok {
		mod.InitConsensusModule(mods, cfg)
	}
}

func (cache *cache) insert(key string) {
	cache.mut.Lock()
	defer cache.mut.Unlock()
	elem, ok := cache.entries[key]
	if ok {
		cache.accessOrder.MoveToFront(elem)
		return
	}
	cache.evict()
	elem = cache.accessOrder.PushFront(key)
	cache.entries[key] = elem
}

func (cache *cache) check(key string) bool {
	cache.mut.Lock()
	defer cache.mut.Unlock()
	elem, ok := cache.entries[key]
	if !ok {
		return false
	}
	cache.accessOrder.MoveToFront(elem)
	return true
}

func (cache *cache) evict() {
	if len(cache.entries) < cache.capacity {
		return
	}
	key := cache.accessOrder.Remove(cache.accessOrder.Back()).(string)
	delete(cache.entries, key)
}

// Sign signs a message and adds it to the cache for use during verification.
func (cache *cache) Sign(message []byte) (sig consensus.QuorumSignature, err error) {
	sig, err = cache.impl.Sign(message)
	if err != nil {
		return nil, err
	}
	var key strings.Builder
	hash := sha256.Sum256(message)
	_, _ = key.Write(hash[:])
	_, _ = key.Write(sig.ToBytes())
	cache.insert(key.String())
	return sig, nil
}

// Verify verifies the given quorum signature against the message.
func (cache *cache) Verify(signature consensus.QuorumSignature, message []byte) bool {
	var key strings.Builder
	hash := sha256.Sum256(message)
	_, _ = key.Write(hash[:])
	_, _ = key.Write(signature.ToBytes())

	if cache.check(key.String()) {
		return true
	}

	if cache.impl.Verify(signature, message) {
		cache.insert(key.String())
		return true
	}

	return false
}

// BatchVerify verifies the given quorum signature against the batch of messages.
func (cache *cache) BatchVerify(signature consensus.QuorumSignature, batch map[hotstuff.ID][]byte) bool {
	// sort the list of ids from the batch map
	ids := maps.Keys(batch)
	slices.Sort(ids)
	var hash consensus.Hash
	hasher := sha256.New()
	// then hash the messages in sorted order
	for _, id := range ids {
		_, _ = hasher.Write(batch[id])
	}
	hasher.Sum(hash[:])

	var key strings.Builder
	_, _ = key.Write(hash[:])
	_, _ = key.Write(signature.ToBytes())

	if cache.check(key.String()) {
		return true
	}

	if cache.impl.BatchVerify(signature, batch) {
		cache.insert(key.String())
		return true
	}

	return false
}

// Combine combines multiple signatures together into a single signature.
func (cache *cache) Combine(signatures ...consensus.QuorumSignature) (consensus.QuorumSignature, error) {
	// we don't cache the result of this operation, because it is not guaranteed to be valid.
	return cache.impl.Combine(signatures...)
}
