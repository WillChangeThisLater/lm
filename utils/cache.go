package utils

import (
        "bytes"
        "crypto/sha256"
        "encoding/gob"
        "fmt"
        "github.com/dgraph-io/badger/v3"
)

type Cache struct {
        db *badger.DB
}

func NewCache(dbPath string) (*Cache, error) {
        opts := badger.DefaultOptions(dbPath)
        opts.Logger = nil
        db, err := badger.Open(opts)
        if err != nil {
                return nil, err
        }
        return &Cache{db: db}, nil
}

func (c *Cache) Close() error {
        return c.db.Close()
}

func (c *Cache) keyHash(key string) string {
        return fmt.Sprintf("%x", sha256.Sum256([]byte(key)))
}

func (c *Cache) Get(key string) (string, error) {
        var result string
        err := c.db.View(func(txn *badger.Txn) error {
                item, err := txn.Get([]byte(c.keyHash(key)))
                if err != nil {
                        return err
                }
                return item.Value(func(val []byte) error {
                        buf := bytes.NewBuffer(val)
                        return gob.NewDecoder(buf).Decode(&result)
                })
        })
        return result, err
}

func (c *Cache) Set(key string, value string) error {
        return c.db.Update(func(txn *badger.Txn) error {
                var buf bytes.Buffer
                err := gob.NewEncoder(&buf).Encode(value)
                if err != nil {
                        return err
                }
                return txn.Set([]byte(c.keyHash(key)), buf.Bytes())
        })
}
