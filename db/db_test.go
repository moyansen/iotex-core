// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"context"
	"math/rand"
	"strconv"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/testutil"
)

var (
	bucket1 = "test_ns1"
	bucket2 = "test_ns2"
	bucket3 = "test_ns3"
	testK1  = [3][]byte{[]byte("key_1"), []byte("key_2"), []byte("key_3")}
	testV1  = [3][]byte{[]byte("value_1"), []byte("value_2"), []byte("value_3")}
	testK2  = [3][]byte{[]byte("key_4"), []byte("key_5"), []byte("key_6")}
	testV2  = [3][]byte{[]byte("value_4"), []byte("value_5"), []byte("value_6")}
	cfg     = config.Default.DB
)

func TestKVStorePutGet(t *testing.T) {
	testKVStorePutGet := func(kvStore KVStore, t *testing.T) {
		assert := assert.New(t)
		ctx := context.Background()

		err := kvStore.Start(ctx)
		assert.Nil(err)
		defer func() {
			err = kvStore.Stop(ctx)
			assert.Nil(err)
		}()

		err = kvStore.Put(bucket1, []byte("key"), []byte("value"))
		assert.Nil(err)
		value, err := kvStore.Get(bucket1, []byte("key"))
		assert.Nil(err)
		assert.Equal([]byte("value"), value)
		value, err = kvStore.Get("test_ns_1", []byte("key"))
		assert.NotNil(err)
		assert.Nil(value)
		value, err = kvStore.Get(bucket1, testK1[0])
		assert.NotNil(err)
		assert.Nil(value)

		err = kvStore.PutIfNotExists(bucket1, testK1[0], testV1[0])
		assert.Nil(err)
		value, err = kvStore.Get(bucket1, testK1[0])
		assert.Nil(err)
		assert.Equal(testV1[0], value)

		err = kvStore.PutIfNotExists(bucket1, testK1[0], testV1[1])
		assert.NotNil(err)
		value, err = kvStore.Get(bucket1, testK1[0])
		assert.Nil(err)
		assert.Equal(testV1[0], value)
	}

	t.Run("In-memory KV Store", func(t *testing.T) {
		testKVStorePutGet(NewMemKVStore(), t)
	})

	path := "/tmp/test-kv-store-" + strconv.Itoa(rand.Int())
	t.Run("Bolt DB", func(t *testing.T) {
		testutil.CleanupPath(t, path)
		defer testutil.CleanupPath(t, path)
		testKVStorePutGet(NewBoltDB(path, cfg), t)
	})
}

func TestBatchRollback(t *testing.T) {
	testBatchRollback := func(kvStore KVStore, t *testing.T) {
		assert := assert.New(t)

		ctx := context.Background()
		kvboltDB := kvStore.(*boltDB)

		err := kvboltDB.Start(ctx)
		assert.Nil(err)
		defer func() {
			err = kvboltDB.Stop(ctx)
			assert.Nil(err)
		}()

		err = kvboltDB.Put(bucket1, testK1[0], testV1[0])
		assert.Nil(err)
		value, err := kvboltDB.Get(bucket1, testK1[0])
		assert.Nil(err)
		assert.Equal(testV1[0], value)
		err = kvboltDB.Put(bucket1, testK1[1], testV1[1])
		assert.Nil(err)
		value, err = kvboltDB.Get(bucket1, testK1[1])
		assert.Nil(err)
		assert.Equal(testV1[1], value)
		err = kvboltDB.Put(bucket1, testK1[2], testV1[2])
		assert.Nil(err)
		value, err = kvboltDB.Get(bucket1, testK1[2])
		assert.Nil(err)
		assert.Equal(testV1[2], value)

		testV := [3][]byte{[]byte("value1.1"), []byte("value2.1"), []byte("value3.1")}

		err = kvboltDB.batchPutForceFail(bucket1, testK1[:], testV[:])
		assert.NotNil(err)

		value, err = kvboltDB.Get(bucket1, testK1[0])
		assert.Nil(err)
		assert.Equal(testV1[0], value)
		value, err = kvboltDB.Get(bucket1, testK1[1])
		assert.Nil(err)
		assert.Equal(testV1[1], value)
		value, err = kvboltDB.Get(bucket1, testK1[2])
		assert.Nil(err)
		assert.Equal(testV1[2], value)
	}

	path := "/tmp/test-batch-rollback-" + strconv.Itoa(rand.Int())
	t.Run("Bolt DB", func(t *testing.T) {
		testutil.CleanupPath(t, path)
		defer testutil.CleanupPath(t, path)
		testBatchRollback(NewBoltDB(path, cfg), t)
	})
}

func TestDBInMemBatchCommit(t *testing.T) {
	require := require.New(t)

	kvStore := NewMemKVStore()

	ctx := context.Background()
	kvboltDB := kvStore.(*memKVStore)
	batch := NewBatch()

	err := kvboltDB.Start(ctx)
	require.Nil(err)
	defer func() {
		err = kvboltDB.Stop(ctx)
		require.Nil(err)
	}()

	err = kvboltDB.Put(bucket1, testK1[0], testV1[1])
	require.Nil(err)

	err = kvboltDB.Put(bucket2, testK2[1], testV2[0])
	require.Nil(err)

	err = kvboltDB.Put(bucket1, testK1[2], testV1[0])
	require.Nil(err)

	batch.Put(bucket1, testK1[0], testV1[0], "")
	err = batch.PutIfNotExists(bucket2, []byte("test"), []byte("test"), "")
	require.Nil(err)

	value, err := kvboltDB.Get(bucket1, testK1[0])
	require.Nil(err)
	require.Equal(testV1[1], value)

	value, err = kvboltDB.Get(bucket2, testK2[1])
	require.Nil(err)
	require.Equal(testV2[0], value)

	err = kvboltDB.Commit(batch)
	require.Nil(err)

	value, err = kvboltDB.Get(bucket1, testK1[0])
	require.Nil(err)
	require.Equal(testV1[0], value)
}

func TestDBBatch(t *testing.T) {
	testBatchRollback := func(kvStore KVStore, t *testing.T) {
		require := require.New(t)

		ctx := context.Background()
		kvboltDB := kvStore.(*boltDB)
		batch := NewBatch()

		err := kvboltDB.Start(ctx)
		require.Nil(err)
		defer func() {
			err = kvboltDB.Stop(ctx)
			require.Nil(err)
		}()

		err = kvboltDB.Put(bucket1, testK1[0], testV1[1])
		require.Nil(err)

		err = kvboltDB.Put(bucket2, testK2[1], testV2[0])
		require.Nil(err)

		err = kvboltDB.Put(bucket1, testK1[2], testV1[0])
		require.Nil(err)

		batch.Put(bucket1, testK1[0], testV1[0], "")
		batch.Put(bucket2, testK2[1], testV2[1], "")
		value, err := kvboltDB.Get(bucket1, testK1[0])
		require.Nil(err)
		require.Equal(testV1[1], value)

		value, err = kvboltDB.Get(bucket2, testK2[1])
		require.Nil(err)
		require.Equal(testV2[0], value)
		require.Nil(kvboltDB.Commit(batch))

		value, err = kvboltDB.Get(bucket1, testK1[0])
		require.Nil(err)
		require.Equal(testV1[0], value)

		value, err = kvboltDB.Get(bucket2, testK2[1])
		require.Nil(err)
		require.Equal(testV2[1], value)

		value, err = kvboltDB.Get(bucket1, testK1[2])
		require.Nil(err)
		require.Equal(testV1[0], value)

		batch.Put(bucket1, testK1[0], testV1[1], "")
		err = batch.PutIfNotExists(bucket2, testK2[1], testV2[0], "")
		require.Nil(err)
		err = kvboltDB.Commit(batch)
		require.Equal(err, ErrAlreadyExist)
		// need to clear the batch in case of commit error
		batch.Clear()

		value, err = kvboltDB.Get(bucket2, testK2[1])
		require.Nil(err)
		require.Equal(testV2[1], value)

		value, err = kvboltDB.Get(bucket1, testK1[0])
		require.Nil(err)
		require.Equal(testV1[0], value)

		err = batch.PutIfNotExists(bucket3, testK2[0], testV2[0], "")
		require.Nil(err)
		err = kvboltDB.Commit(batch)
		require.Nil(err)

		value, err = kvboltDB.Get(bucket3, testK2[0])
		require.Nil(err)
		require.Equal(testV2[0], value)

		batch.Put(bucket1, testK1[2], testV1[2], "")
		// we did not set key in bucket3 yet, so this operation will fail and
		// cause transaction rollback
		err = batch.PutIfNotExists(bucket3, testK2[0], testV2[1], "")
		require.Nil(err)
		require.NotNil(kvboltDB.Commit(batch))

		value, err = kvboltDB.Get(bucket1, testK1[2])
		require.Nil(err)
		require.Equal(testV1[0], value)

		value, err = kvboltDB.Get(bucket2, testK2[1])
		require.Nil(err)
		require.Equal(testV2[1], value)

		batch.Clear()
		batch.Put(bucket1, testK1[2], testV1[2], "")
		batch.Delete(bucket2, testK2[1], "")
		require.Nil(kvboltDB.Commit(batch))

		value, err = kvboltDB.Get(bucket1, testK1[2])
		require.Nil(err)
		require.Equal(testV1[2], value)

		_, err = kvboltDB.Get(bucket2, testK2[1])
		require.NotNil(err)
	}

	t.Run("Bolt DB", func(t *testing.T) {
		path := "/tmp/test-batch-rollback-" + strconv.Itoa(rand.Int())
		testutil.CleanupPath(t, path)
		defer testutil.CleanupPath(t, path)
		testBatchRollback(NewBoltDB(path, cfg), t)
	})
}

func TestCacheKV(t *testing.T) {
	testFunc := func(kv KVStore, t *testing.T) {
		require := require.New(t)

		cb := NewCachedBatch()
		cb.Put(bucket1, testK1[0], testV1[0], "")
		v, _ := cb.Get(bucket1, testK1[0])
		require.Equal(testV1[0], v)
		cb.Clear()
		_, err := cb.Get(bucket1, testK1[0])
		require.Error(err)
		cb.Put(bucket2, testK2[2], testV2[2], "")
		v, _ = cb.Get(bucket2, testK2[2])
		require.Equal(testV2[2], v)
		// <k1[0], v1[0]> is gone
		require.Nil(cb.PutIfNotExists(bucket1, testK1[0], testV1[0], ""))
		require.Nil(cb.PutIfNotExists(bucket1, testK1[2], testV1[2], ""))
		require.Nil(cb.PutIfNotExists(bucket1, testK1[1], testV1[1], ""))
		v, _ = cb.Get(bucket1, testK1[0])
		require.Equal(testV1[0], v)
		v, _ = cb.Get(bucket1, testK1[1])
		require.Equal(testV1[1], v)
		v, _ = cb.Get(bucket1, testK1[2])
		require.Equal(testV1[2], v)
		// put testK1[1] with a new value
		cb.Put(bucket1, testK1[1], testV1[2], "")
		v, _ = cb.Get(bucket1, testK1[1])
		require.Equal(testV1[2], v)
		// cannot put same entry again
		require.Equal(ErrAlreadyExist, errors.Cause(cb.PutIfNotExists(bucket1, testK1[1], testV1[0], "")))
		// same key but diff bucket name is OK
		require.Nil(cb.PutIfNotExists(bucket2, testK1[0], testV1[0], ""))
		// delete a non-existing entry is OK
		cb.Delete(bucket2, []byte("notexist"), "")
		require.Nil(kv.Commit(cb))

		cb = nil
		cb = NewCachedBatch()
		v, _ = kv.Get(bucket1, testK1[0])
		require.Equal(testV1[0], v)
		v, _ = kv.Get(bucket1, testK1[1])
		require.Equal(testV1[2], v)
		v, _ = kv.Get(bucket1, testK1[2])
		require.Equal(testV1[2], v)
		v, _ = kv.Get(bucket2, testK1[0])
		require.Equal(testV1[0], v)
		require.Nil(cb.PutIfNotExists(bucket2, testK2[0], testV2[0], ""))
		require.NoError(kv.Commit(cb))
		// entry exists in DB but not in cache, so OK at this point
		require.Nil(cb.PutIfNotExists(bucket2, testK1[0], testV1[2], ""))
		// but would fail upon commit
		require.Equal(ErrAlreadyExist, errors.Cause(kv.Commit(cb)))
	}

	t.Run("In-memory KV Store", func(t *testing.T) {
		kv := NewMemKVStore()
		testFunc(kv, t)
	})

	path := "db.test"
	t.Run("Bolt DB", func(t *testing.T) {
		testutil.CleanupPath(t, path)
		defer testutil.CleanupPath(t, path)
		kv := NewBoltDB(path, cfg)
		require.Nil(t, kv.Start(context.Background()))
		defer func() {
			err := kv.Stop(context.Background())
			require.Nil(t, err)
		}()
		testFunc(kv, t)
	})
}

func TestCachedBatch(t *testing.T) {
	require := require.New(t)

	cb := NewCachedBatch()
	cb.Put(bucket1, testK1[0], testV1[0], "")
	v, err := cb.Get(bucket1, testK1[0])
	require.NoError(err)
	require.Equal(testV1[0], v)
	require.Equal(ErrAlreadyExist, cb.PutIfNotExists(bucket1, testK1[0], testV1[0], ""))
	v, err = cb.Get(bucket1, testK2[0])
	require.Equal(ErrNotExist, err)
	require.Equal([]byte(nil), v)

	cb.Delete(bucket1, testK2[0], "")
	cb.Delete(bucket1, testK1[0], "")
	require.NoError(cb.PutIfNotExists(bucket1, testK1[0], testV1[0], ""))
	v, err = cb.Get(bucket1, testK1[0])
	require.NoError(err)
	require.Equal(testV1[0], v)

	w, err := cb.Entry(1)
	require.NoError(err)
	require.Equal(bucket1, w.namespace)
	require.Equal(testK2[0], w.key)
	require.Equal([]byte(nil), w.value)
	require.Equal(Delete, w.writeType)

	w, err = cb.Entry(3)
	require.NoError(err)
	require.Equal(bucket1, w.namespace)
	require.Equal(testK1[0], w.key)
	require.Equal(testV1[0], w.value)
	require.Equal(PutIfNotExists, w.writeType)
}
