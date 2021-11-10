package transfer

import (
	"fmt"
	"log"
	"time"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/iterator"
)

// Transfer ...
type Transfer struct {
	bq     *BigQueryClient
	pg     *PostgresClient
	config *TransferConfig
}

// NewTransfer ...
func NewTransfer(bq *BigQueryClient, pg *PostgresClient, config *TransferConfig) *Transfer {
	return &Transfer{
		bq:     bq,
		pg:     pg,
		config: config,
	}
}

// Run ...
func (t *Transfer) Run() {
	activeTransfers := 0
	done := make(chan bool)

	for _, dataset := range t.config.Datasets {
		for _, table := range dataset.Tables {
			go t.transferTable(dataset.Name, table.Name, table.DateField, done)
			activeTransfers++
		}
	}
	for activeTransfers > 0 {
		log.Printf("Waiting for done, activeTransfers: %d", activeTransfers)
		<-done
		activeTransfers--
		log.Printf("Transfer done, activeTransfers: %d", activeTransfers)
	}

	log.Println("All transfers complete")
}

// RunContinuously continuously runs Transfer.Run, with sleepTimeInSecs between
// transfers
func (t *Transfer) RunContinuously(sleepAfterTransferInSecs int) {
	for {
		t.Run()
		log.Printf("Sleeping for %d seconds", sleepAfterTransferInSecs)
		time.Sleep(time.Duration(sleepAfterTransferInSecs) * time.Second)
	}
}

func (t *Transfer) transferTable(bigQueryDataset, tableName, dateField string, done chan bool) {
	// Check if table already exists in the Postgres instance
	tableExists := t.pg.TableExists(tableName)

	// Get the BigQuery table schema
	schema, err := t.bq.GetTableSchema(bigQueryDataset, tableName)
	if err != nil {
		log.Fatalf("Could not get schema for table %s: %v", tableName, err)
	}

	// Create table if needed, using schema of BigQuery table
	if tableExists == false {
		log.Printf("Table %s not found in Postgres, creating...", tableName)
		err = t.pg.CreateTableFromBQSchema(tableName, schema)
		if err != nil {
			log.Fatalf("Could not get create table: %s", err)
		}
	}

	// From Postgres table, get most recent entry
	var pgTableEmpty bool
	timestamp, err := t.pg.GetMostRecentEntry(tableName, dateField)
	if err != nil {
		log.Fatalf("Could not get most recent Postgres timestamp: %s", err)
	}
	if timestamp == "" {
		pgTableEmpty = true
	}

	// Get required rows from BigQuery
	var rows *bigquery.RowIterator
	if pgTableEmpty {
		log.Println("Table empty, getting all data")
		rows, err = t.bq.GetAllData(bigQueryDataset, tableName, schema)
	} else {
		log.Printf("Need data occuring after: %s\n", timestamp)
		rows, err = t.bq.GetDataAfterDatetime(bigQueryDataset, tableName, dateField, timestamp, schema)
	}
	if err != nil {
		log.Fatalf("Could not get data from big query: %s", err) // TODO: which table? etc
	}

	// Begin transaction
	ctx := t.pg.ctx
	tx, err := t.pg.Begin(ctx)
	if err != nil {
		log.Fatalf("Could not begin transaction for table %s: %s", tableName, err)
	}
	defer tx.Rollback(ctx)

	// Transfer rows to Postgres
	successes, failures := 0, 0
	rowsPrinted := false
	for {
		row := make(map[string]bigquery.Value)
		err = rows.Next(&row)
		if !rowsPrinted {
			log.Printf("Rows to process for table %s: %d", tableName, rows.TotalRows)
			rowsPrinted = true
		}
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Printf("Big query row error: %s", err)
			failures++
			continue
		}

		insertSQL, err := createInsertSQL(tableName, schema, row)
		if err != nil {
			log.Printf("Could not construct insert SQL: %s", err)
			failures++
			continue
		}

		_, err = tx.Exec(ctx, insertSQL)
		if err != nil {
			log.Printf("Transaction exec error: %s", err)
			log.Fatal(insertSQL)
		}
		successes++
	}

	err = tx.Commit(ctx)
	if err != nil {
		log.Fatalf("Transaction commit error: %s", err)
	}

	log.Printf("Finished processing table '%s'. Rows transfered: %d/%d", tableName, successes, successes+failures)
	done <- true
}

func createInsertSQL(tableName string, schema map[string]string, row map[string]bigquery.Value) (string, error) {
	columnNames := ""
	valuesString := ""

	for colName := range schema {
		if columnNames == "" {
			columnNames = fmt.Sprintf("%s", colName)
		} else {
			columnNames = fmt.Sprintf("%s, %s", columnNames, colName)
		}

		colValue := row[colName]
		if schema[colName] == "TIMESTAMPTZ" {
			colValue = row[colName].(time.Time).Format(time.RFC3339)
		}
		if schema[colName] == "DOUBLE PRECISION" {
			if colValue == nil {
				// TODO: Change to "NULL" after addding support for nullable columns.
				colValue = "0"
			} else {
				colValue = fmt.Sprintf("%f", row[colName])
			}

		}
		if valuesString == "" {
			valuesString = fmt.Sprintf(`'%s'`, colValue)
		} else {
			valuesString = fmt.Sprintf(`%s, '%s'`, valuesString, colValue)
		}
	}

	insertSQL := fmt.Sprintf(`INSERT INTO "%s" (%s) VALUES (%s);`, tableName, columnNames, valuesString)
	return insertSQL, nil
}
