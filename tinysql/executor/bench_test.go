package executor_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/session"
	"github.com/pingcap/tidb/store/mockstore"
)

type BenchKit struct {
	b     *testing.B
	store kv.Storage
	se    session.Session
}

func NewBenchKit(b *testing.B, store kv.Storage) *BenchKit {
	return &BenchKit{
		b:     b,
		store: store,
	}
}

func (bk *BenchKit) MustExec(sql string) {
	if bk.se == nil {
		se, err := session.CreateSession4Test(bk.store)
		if err != nil {
			bk.b.Fatal(err)
		}
		bk.se = se
	}
	ctx := context.Background()
	rss, err := bk.se.Execute(ctx, sql)
	if err != nil {
		bk.b.Fatal(err)
	}
	if len(rss) > 0 {
		rss[0].Close()
	}
}

func BenchmarkSimpleSelect(b *testing.B) {
	store, err := mockstore.NewMockTikvStore()
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	session.SetSchemaLease(0)
	session.DisableStats4Test()
	dom, err := session.BootstrapSession(store)
	if err != nil {
		b.Fatal(err)
	}
	defer dom.Close()

	tk := NewBenchKit(b, store)
	tk.MustExec("use test")
	tk.MustExec("create table t (id int primary key, v int)")
	tk.MustExec("insert into t values (1, 1), (2, 2), (3, 3)")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tk.MustExec("select * from t where id = 1")
	}
}

func BenchmarkAggregation(b *testing.B) {
	store, err := mockstore.NewMockTikvStore()
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	session.SetSchemaLease(0)
	session.DisableStats4Test()
	dom, err := session.BootstrapSession(store)
	if err != nil {
		b.Fatal(err)
	}
	defer dom.Close()

	tk := NewBenchKit(b, store)
	tk.MustExec("use test")
	tk.MustExec("create table t (id int primary key, v int)")
	for i := 0; i < 100; i++ {
		tk.MustExec(fmt.Sprintf("insert into t values (%d, %d)", i, i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tk.MustExec("select count(*) from t")
	}
}
