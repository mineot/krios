```go
client, err := krios.NewClient(...)
// fazer todos os tratamentos necessarios...

s1 := client.Select().
    From("table1", "t1", "public").
    From("table2", "t2", "public")

s2 := krios.Select{
    Client: client,
    From: []krios.From{
        {Name: "table1", Alias: "t1", Schema: "public"},
        {Name: "table2", Alias: "t2", Schema: "public"},
    },
    Columns: []krios.Column{
        {Name: "col1", Alias: "c1", Table: "t1", Aggregate: krios.SUM},
    }
}
```
