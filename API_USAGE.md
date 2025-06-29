# API Usage

## Column Patch

`ColumnPatch` allows partial updates to a column. Each field is optional; if the pointer is nil the value is unchanged.

```go
patch := gcfmmodel.ColumnPatch{
    Label:       strPtr("Name"),
    Index:       boolPtr(true), // create index idx_<column>
    Unique:      boolPtr(true), // add unique constraint unique_<column>
}
err := client.PatchColumn(ctx, "table", "column", patch)

// remove the unique constraint later
patch = gcfmmodel.ColumnPatch{Unique: boolPtr(false)}
err = client.PatchColumn(ctx, "table", "column", patch)
```

## ChangeSet

Use `BeginChangeSet` to batch multiple operations. All patches are executed within a single transaction when `Commit` is called.

```go
cs := client.BeginChangeSet()
cs.PatchColumn(ctx, "table", "col1", patch1)
cs.PatchColumn(ctx, "table", "col2", patch2)
if err := cs.Commit(ctx); err != nil {
    // rolled back automatically
}
// the ChangeSet becomes unusable after Commit.
// Its internal client reference is cleared to prevent accidental reuse.
```

## DBManagementClient

`DBManagementClient` provides high level helpers to manage tables and columns.
Create one using your controllers and call its methods:

```go
dbm := client.NewDBManagementClient(modelCtl, initCtl, uninstallCtl,
    showCtl, updateCtl, rollbackCtl, validationCtl)

// create a table
params := gcfmmodel.AddTableParams{TableName: "users", ColumnDefs: cols}
if err := dbm.AddTable(ctx, params); err != nil {
    log.Fatal(err)
}

// update a column using a patch
patch := gcfmmodel.ColumnPatch{Label: strPtr("User Name")}
if err := dbm.PatchColumn(ctx, "users", "name", patch); err != nil {
    log.Fatal(err)
}

// batch operations
cs := dbm.BeginChangeSet()
cs.PatchColumn(ctx, "users", "name", patch)
if err := cs.Commit(ctx); err != nil {
    log.Fatal(err)
}
```
