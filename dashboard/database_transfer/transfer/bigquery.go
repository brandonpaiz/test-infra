package transfer

import (
	"context"
	"fmt"
	"log"
	"strings"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/iterator"
)

// BigQueryClient interacts with an instance of BigQuery.
type BigQueryClient struct {
	bqClient *bigquery.Client
	ctx      context.Context
}

// NewBigQueryClient creates a new BigQueryClient.
func NewBigQueryClient(ctx context.Context, config BigQueryConfig) (*BigQueryClient, error) {
	bq, err := bigquery.NewClient(ctx, config.ProjectID)
	if err != nil {
		return nil, err
	}
	bqc := &BigQueryClient{bq, ctx}
	if err != nil {
		return nil, err
	}
	return bqc, nil
}

// ListTables lists all tables in the BigQuery instance
func (bqc *BigQueryClient) ListTables() error {
	it := bqc.bqClient.Datasets(bqc.ctx)
	for {
		dataset, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		log.Println(dataset.DatasetID)
	}
	return nil
}

// GetAllData gets all data from a BigQuery table
func (bqc *BigQueryClient) GetAllData(dataset, table string, schema map[string]string) (*bigquery.RowIterator, error) {
	return bqc.GetDataAfterDatetime(dataset, table, "", "", schema)
}

// GetDataAfterDatetime gets all data after the specified datetime.
func (bqc *BigQueryClient) GetDataAfterDatetime(dataset, table, dateField, datetime string, schema map[string]string) (*bigquery.RowIterator, error) {
	selectStr := ""
	for columnName, dataType := range schema {
		var selectCol string
		if dataType == "JSON" {
			selectCol = fmt.Sprintf("TO_JSON_STRING(%s) AS %s", columnName, columnName)
		} else {
			selectCol = fmt.Sprintf("%s", columnName)
		}

		if selectStr == "" {
			selectStr = selectCol
		} else {
			selectStr = fmt.Sprintf("%s, %s", selectStr, selectCol)
		}
	}
	querySQL := fmt.Sprintf("SELECT %s FROM `%s.%s` WHERE %s > \"%s\";", selectStr, dataset, table, dateField, datetime)
	if datetime == "" {
		querySQL = fmt.Sprintf("SELECT %s FROM `%s.%s`;", selectStr, dataset, table)
	}
	log.Print(querySQL)

	return bqc.bqClient.Query(querySQL).Read(bqc.ctx)
}

// GetTableSchema gets the schema for the specified BigQuery table. It returns
// a map whose keys are column names and values are PostgreSQL types.
func (bqc *BigQueryClient) GetTableSchema(dataset, table string) (map[string]string, error) {
	schema := make(map[string]string)

	colQuery := "SELECT column_name, data_type FROM `%s.INFORMATION_SCHEMA.COLUMNS` WHERE table_name=\"%s\""
	colQuery = fmt.Sprintf(colQuery, dataset, table)

	query := bqc.bqClient.Query(colQuery)
	rows, err := query.Read(bqc.ctx)
	if err != nil {
		return nil, err
	}

	for {
		var row RowSchema
		err := rows.Next(&row)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		// TODO: make BigQuerySchema type, return that, then do this transformation elsewhere. like in transfer
		if strings.Contains(row.DataType, "STRUCT") {
			row.DataType = "JSON"
		}
		if strings.Contains(row.DataType, "FLOAT64") {
			row.DataType = "DOUBLE PRECISION"
		}
		if strings.Contains(row.DataType, "STRING") {
			row.DataType = "TEXT"
		}
		if strings.Contains(row.DataType, "TIME") {
			row.DataType = "TIMESTAMPTZ"
		}
		schema[row.ColumnName] = row.DataType
	}

	return schema, nil
}

// RowSchema associates a column name with its BigQuery data type
type RowSchema struct {
	ColumnName string `bigquery:"column_name"`
	DataType   string `bigquery:"data_type"`
}
