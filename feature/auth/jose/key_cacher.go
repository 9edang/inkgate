package jose

import (
	b64 "encoding/base64"
	"errors"
	"sync"
	"time"

	"github.com/9edang/inkgate/internal/json"

	"github.com/go-jose/go-jose/v3"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
)

// key identification strategies: which JWK field the cached keys are indexed by.
const (
	strategyKid     = "kid"
	strategyX5t     = "x5t"
	strategyX5tS256 = "x5t#S256"
	strategyKidX5t  = "kid_x5t"
)

var (
	ErrNoKeyFound                   = errors.New("no Keys have been found")
	ErrKeyExpired                   = errors.New("key exists but is expired")
	defaultGlobalCacheMaxAge uint32 = 900
	defaultStrategy                 = strategyKid

	// Configuring with MaxKeyAgeNoCheck will skip key expiry check.
	MaxKeyAgeNoCheck    = time.Duration(-1)
	globalKeyCacher     = map[string]GlobalCacher{}
	globalKeyCacherOnce = new(sync.Once)
)

type GlobalCacher struct {
	kc KeyCacher
	mu *sync.RWMutex
}

func SetGlobalCacher(l logging.Logger, cfg config.ExtraConfig) error {
	scfg, err := configGetter(l, cfg)
	if err != nil {
		return err
	}
	duration := time.Duration(scfg.CacheDuration) * time.Second
	globalKeyCacherOnce.Do(func() {
		globalKeyCacher = map[string]GlobalCacher{
			strategyKid:     {kc: NewMemoryKeyCacher(duration, -1, strategyKid), mu: new(sync.RWMutex)},
			strategyX5t:     {kc: NewMemoryKeyCacher(duration, -1, strategyX5t), mu: new(sync.RWMutex)},
			strategyX5tS256: {kc: NewMemoryKeyCacher(duration, -1, strategyX5tS256), mu: new(sync.RWMutex)},
			strategyKidX5t:  {kc: NewMemoryKeyCacher(duration, -1, strategyKidX5t), mu: new(sync.RWMutex)},
		}
	})

	return nil
}

type serviceConfig struct {
	CacheDuration uint32 `json:"shared_cache_duration"`
}

func configGetter(l logging.Logger, cfg config.ExtraConfig) (serviceConfig, error) {
	scfg := serviceConfig{}
	e, ok := cfg[ValidatorNamespace].(map[string]any)
	if !ok {
		return scfg, ErrNoValidatorCfg
	}
	tmp, err := json.Marshal(e)
	if err != nil {
		return scfg, err
	}
	if err := json.Unmarshal(tmp, &scfg); err != nil {
		return scfg, err
	}
	if scfg.CacheDuration == 0 {
		scfg.CacheDuration = defaultGlobalCacheMaxAge
		l.Info("[SERVICE: JOSE] Empty shared_cache_duration, using default (15m)")
	}

	return scfg, nil
}

// KeyIDGetter extracts a key id from a JSONWebKey.
type KeyIDGetter interface {
	Get(*jose.JSONWebKey) string
}

// KeyIDGetterFunc function conforming to the KeyIDGetter interface.
type KeyIDGetterFunc func(*jose.JSONWebKey) string

// Get calls f(r).
func (f KeyIDGetterFunc) Get(key *jose.JSONWebKey) string {
	return f(key)
}

// DefaultKeyIDGetter returns the default kid as JSONWebKey key id.
func DefaultKeyIDGetter(key *jose.JSONWebKey) string {
	return key.KeyID
}

// X5TKeyIDGetter extracts the key id from the jSONWebKey as the x5t.
func X5TKeyIDGetter(key *jose.JSONWebKey) string {
	return b64.RawURLEncoding.EncodeToString(key.CertificateThumbprintSHA1)
}

// X5TS256KeyIDGetter extracts the key id from the jSONWebKey as the x5t#S256.
func X5TS256KeyIDGetter(key *jose.JSONWebKey) string {
	return b64.RawURLEncoding.EncodeToString(key.CertificateThumbprintSHA256)
}

// CompoundX5TKeyIDGetter extracts the key id from the jSONWebKey as the a compound string of the kid and the x5t.
func CompoundX5TKeyIDGetter(key *jose.JSONWebKey) string {
	return key.KeyID + X5TKeyIDGetter(key)
}

func KeyIDGetterFactory(keyIdentifyStrategy string) KeyIDGetter {
	supportedKeyIdentifyStrategy := map[string]KeyIDGetterFunc{
		strategyKid:     DefaultKeyIDGetter,
		strategyX5t:     X5TKeyIDGetter,
		strategyX5tS256: X5TS256KeyIDGetter,
		strategyKidX5t:  CompoundX5TKeyIDGetter,
	}

	if keyGetter, ok := supportedKeyIdentifyStrategy[keyIdentifyStrategy]; ok {
		return keyGetter
	}

	return KeyIDGetterFunc(DefaultKeyIDGetter)
}

type KeyCacher interface {
	Get(keyID string) (*jose.JSONWebKey, error)
	Add(keyID string, webKeys []jose.JSONWebKey) (*jose.JSONWebKey, error)
}

type MemoryKeyCacher struct {
	entries      map[string]keyCacherEntry
	maxKeyAge    time.Duration
	maxCacheSize int
	keyIDGetter  KeyIDGetter
	mu           *sync.RWMutex
}

type keyCacherEntry struct {
	addedAt time.Time
	jose.JSONWebKey
}

type GMemoryKeyCacher struct {
	*MemoryKeyCacher
	Global GlobalCacher
}

func (gkc *GMemoryKeyCacher) Add(keyID string, downloadedKeys []jose.JSONWebKey) (*jose.JSONWebKey, error) {
	if gkc.Global.kc != nil {
		gkc.Global.mu.Lock()
		// #nosec G104 -- error is not actionable here
		_, _ = gkc.Global.kc.Add(keyID, downloadedKeys)
		gkc.Global.mu.Unlock()
	}

	return gkc.MemoryKeyCacher.Add(keyID, downloadedKeys)
}

// Get obtains a key from the cache, and checks if the key is expired.
func (gkc *GMemoryKeyCacher) Get(keyID string) (*jose.JSONWebKey, error) {
	k, err := gkc.MemoryKeyCacher.Get(keyID)
	if err == nil || gkc.Global.kc == nil {
		return k, err
	}

	gkc.Global.mu.RLock()
	v, err := gkc.Global.kc.Get(keyID)
	gkc.Global.mu.RUnlock()
	if err == nil {
		// #nosec G104 -- error is not actionable here
		_, _ = gkc.MemoryKeyCacher.Add(keyID, []jose.JSONWebKey{*v})
	}

	return v, err
}

// NewMemoryKeyCacher creates a new Keycacher interface with option
// to set max age of cached keys and max size of the cache.
func NewMemoryKeyCacher(maxKeyAge time.Duration, maxCacheSize int, keyIdentifyStrategy string) *MemoryKeyCacher {
	return &MemoryKeyCacher{
		entries:      map[string]keyCacherEntry{},
		maxKeyAge:    maxKeyAge,
		maxCacheSize: maxCacheSize,
		keyIDGetter:  KeyIDGetterFactory(keyIdentifyStrategy),
		mu:           new(sync.RWMutex),
	}
}

func NewGlobalMemoryKeyCacher(maxKeyAge time.Duration, maxCacheSize int, keyIdentifyStrategy string) *GMemoryKeyCacher {
	kc := &GMemoryKeyCacher{
		MemoryKeyCacher: NewMemoryKeyCacher(maxKeyAge, maxCacheSize, keyIdentifyStrategy),
		Global:          GlobalCacher{},
	}
	if keyIdentifyStrategy == "" {
		keyIdentifyStrategy = defaultStrategy
	}
	if len(globalKeyCacher) > 0 {
		g := globalKeyCacher[keyIdentifyStrategy]
		kc.Global = g
	}

	return kc
}

// Get obtains a key from the cache, and checks if the key is expired.
func (mkc *MemoryKeyCacher) Get(keyID string) (*jose.JSONWebKey, error) {
	mkc.mu.RLock()
	searchKey, ok := mkc.entries[keyID]
	mkc.mu.RUnlock()
	if ok {
		if mkc.maxKeyAge == MaxKeyAgeNoCheck || !mkc.keyIsExpired(keyID) {
			return &searchKey.JSONWebKey, nil
		}

		return nil, ErrKeyExpired
	}

	return nil, ErrNoKeyFound
}

// Add adds a key into the cache and handles overflow.
func (mkc *MemoryKeyCacher) Add(keyID string, downloadedKeys []jose.JSONWebKey) (*jose.JSONWebKey, error) {
	var addingKey jose.JSONWebKey
	var addingKeyID string
	for i := range downloadedKeys {
		cacheKey := mkc.keyIDGetter.Get(&downloadedKeys[i])
		if cacheKey == keyID {
			addingKey = downloadedKeys[i]
			addingKeyID = cacheKey
		}
		if mkc.maxCacheSize == -1 {
			mkc.mu.Lock()
			mkc.entries[cacheKey] = keyCacherEntry{
				addedAt:    time.Now(),
				JSONWebKey: downloadedKeys[i],
			}
			mkc.mu.Unlock()
		}
	}
	if addingKey.Key != nil {
		if mkc.maxCacheSize != -1 {
			mkc.mu.Lock()
			mkc.entries[addingKeyID] = keyCacherEntry{
				addedAt:    time.Now(),
				JSONWebKey: addingKey,
			}
			mkc.mu.Unlock()
			mkc.handleOverflow()
		}

		return &addingKey, nil
	}

	return nil, ErrNoKeyFound
}

// keyIsExpired deletes the key from cache if it is expired.
func (mkc *MemoryKeyCacher) keyIsExpired(keyID string) bool {
	mkc.mu.RLock()
	entry := mkc.entries[keyID].addedAt.Add(mkc.maxKeyAge)
	mkc.mu.RUnlock()

	if time.Now().After(entry) {
		mkc.mu.Lock()
		delete(mkc.entries, keyID)
		mkc.mu.Unlock()

		return true
	}

	return false
}

// handleOverflow deletes the oldest key from the cache if overflowed.
func (mkc *MemoryKeyCacher) handleOverflow() {
	if mkc.maxCacheSize < len(mkc.entries) {
		mkc.mu.RLock()
		var oldestEntryKeyID string
		latestAddedTime := time.Now()
		for entryKeyID := range mkc.entries {
			if mkc.entries[entryKeyID].addedAt.Before(latestAddedTime) {
				latestAddedTime = mkc.entries[entryKeyID].addedAt
				oldestEntryKeyID = entryKeyID
			}
		}
		mkc.mu.RUnlock()

		mkc.mu.Lock()
		delete(mkc.entries, oldestEntryKeyID)
		mkc.mu.Unlock()
	}
}
