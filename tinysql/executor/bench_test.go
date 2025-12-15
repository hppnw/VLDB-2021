package executor_test

import (
	"context"
	"fmt"
	"testing"
	"time"

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

// BenchmarkSelectWithoutIndex tests query performance without index
func BenchmarkSelectWithoutIndex(b *testing.B) {
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
	tk.MustExec("create table users (id int primary key, name varchar(50), age int, city varchar(50))")
	
	// Insert 1000 rows
	for i := 0; i < 1000; i++ {
		tk.MustExec(fmt.Sprintf("insert into users values (%d, 'user%d', %d, 'city%d')", 
			i, i, 20+i%50, i%10))
	}

	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		tk.MustExec("select * from users where age = 25")
	}
	elapsed := time.Since(start)
	
	throughput := float64(b.N) / elapsed.Seconds()
	b.ReportMetric(throughput, "ops/sec")
}

// BenchmarkSelectWithIndex tests query performance with index
func BenchmarkSelectWithIndex(b *testing.B) {
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
	tk.MustExec("create table users (id int primary key, name varchar(50), age int, city varchar(50))")
	
	// Insert 1000 rows
	for i := 0; i < 1000; i++ {
		tk.MustExec(fmt.Sprintf("insert into users values (%d, 'user%d', %d, 'city%d')", 
			i, i, 20+i%50, i%10))
	}
	
	// Add index on age column
	tk.MustExec("create index idx_age on users(age)")

	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		tk.MustExec("select * from users where age = 25")
	}
	elapsed := time.Since(start)
	
	throughput := float64(b.N) / elapsed.Seconds()
	b.ReportMetric(throughput, "ops/sec")
}

// BenchmarkAggregationWithIndex tests aggregation with indexed column
func BenchmarkAggregationWithIndex(b *testing.B) {
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
	tk.MustExec("create table orders (id int primary key, user_id int, amount bigint, status int)")
	
	// Insert 1000 rows
	for i := 0; i < 1000; i++ {
		tk.MustExec(fmt.Sprintf("insert into orders values (%d, %d, %d, %d)", 
			i, i%100, 100+i%500, i%3))
	}
	
	// Add composite index
	tk.MustExec("create index idx_user_status on orders(user_id, status)")

	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		tk.MustExec("select user_id, sum(amount) from orders where status = 1 group by user_id")
	}
	elapsed := time.Since(start)
	
	throughput := float64(b.N) / elapsed.Seconds()
	b.ReportMetric(throughput, "ops/sec")
}

// BenchmarkJoinWithIndex tests join performance with indexes
func BenchmarkJoinWithIndex(b *testing.B) {
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
	tk.MustExec("create table users (id int primary key, name varchar(50))")
	tk.MustExec("create table orders (id int primary key, user_id int, amount bigint)")
	
	// Insert data
	for i := 0; i < 100; i++ {
		tk.MustExec(fmt.Sprintf("insert into users values (%d, 'user%d')", i, i))
	}
	for i := 0; i < 500; i++ {
		tk.MustExec(fmt.Sprintf("insert into orders values (%d, %d, %d)", i, i%100, 100+i))
	}
	
	// Add index on foreign key
	tk.MustExec("create index idx_user_id on orders(user_id)")

	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		tk.MustExec("select u.name, sum(o.amount) from users u join orders o on u.id = o.user_id group by u.id, u.name")
	}
	elapsed := time.Since(start)
	
	throughput := float64(b.N) / elapsed.Seconds()
	b.ReportMetric(throughput, "ops/sec")
}

// BenchmarkBatchInsert tests batch insert performance
func BenchmarkBatchInsert(b *testing.B) {
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
	tk.MustExec("create table batch_test (id int primary key, value int)")

	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		// Batch insert 10 rows at once
		values := ""
		for j := 0; j < 10; j++ {
			if j > 0 {
				values += ","
			}
			values += fmt.Sprintf("(%d, %d)", i*10+j, i*10+j)
		}
		tk.MustExec("insert into batch_test values " + values)
		tk.MustExec("delete from batch_test where id >= " + fmt.Sprintf("%d", i*10))
	}
	elapsed := time.Since(start)
	
	throughput := float64(b.N*10) / elapsed.Seconds() // Insert 10 rows per iteration
	b.ReportMetric(throughput, "rows/sec")
}

// BenchmarkCompositeIndex tests composite index vs single column index
func BenchmarkCompositeIndex(b *testing.B) {
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
	tk.MustExec("create table products (id int primary key, category int, price int, stock int)")
	
	// Insert 1000 rows
	for i := 0; i < 1000; i++ {
		tk.MustExec(fmt.Sprintf("insert into products values (%d, %d, %d, %d)", 
			i, i%10, 100+i%500, 50+i%100))
	}
	
	// Create composite index (category, price)
	tk.MustExec("create index idx_cat_price on products(category, price)")

	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		// Query that benefits from composite index
		tk.MustExec("select * from products where category = 5 and price > 200")
	}
	elapsed := time.Since(start)
	
	throughput := float64(b.N) / elapsed.Seconds()
	b.ReportMetric(throughput, "ops/sec")
}
