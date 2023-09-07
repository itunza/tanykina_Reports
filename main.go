package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/xuri/excelize/v2"
)

type Record struct {
	SupplierName    string
	TransactionDate time.Time
	BroughtMilk     float64
}

type DateRange struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

func main() {
	// load the .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	http.HandleFunc("/insert", insertHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func insertHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	var dateRange DateRange
	err := json.NewDecoder(r.Body).Decode(&dateRange)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	connStr := fmt.Sprintf("host=%s port=%s user=%s dbname=%s password=%s sslmode=disable", os.Getenv("DATABASE_HOST"), os.Getenv("DATABASE_PORT"), os.Getenv("DATABASE_USER"), os.Getenv("DATABASE_NAME"), os.Getenv("DATABASE_PASSWORD"))

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}

	query := fmt.Sprintf(`WITH DateSeries AS (
		SELECT generate_series(
		  '%s'::date, 
		  '%s'::date, 
		  '1 day'::interval
		) AS transaction_date
	  ),
	  AllFarmerDates AS (
		SELECT DISTINCT t.sno, ds.transaction_date
		FROM transactions t
		CROSS JOIN DateSeries ds
	  )
	  SELECT afd.sno, 
			 afd.transaction_date,
			 COALESCE(t.total_qty, 0) AS TotalQty
	  FROM AllFarmerDates afd
	  LEFT JOIN transactions t
	  ON afd.sno = t.sno AND afd.transaction_date = DATE(t.transaction_date)
	  ORDER BY afd.sno, afd.transaction_date;
	  `, dateRange.StartDate, dateRange.EndDate)

	rows, err := db.Query(query)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	var records []Record

	for rows.Next() {
		var r Record
		if err := rows.Scan(&r.SupplierName, &r.TransactionDate, &r.BroughtMilk); err != nil {
			log.Fatal(err)
		}
		records = append(records, r)
	}

	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}

	f := excelize.NewFile()
	f.SetCellValue("Sheet1", "A1", "Supplier Name")

	supplierRow := make(map[string]int)
	dateColumnMap := make(map[string]int)

	for _, record := range records {
		if _, exists := supplierRow[record.SupplierName]; !exists {
			rowNum := len(supplierRow) + 2
			supplierRow[record.SupplierName] = rowNum
			f.SetCellValue("Sheet1", fmt.Sprintf("A%d", rowNum), record.SupplierName)
		}

		dateStr := record.TransactionDate.Format("2006-01-02")
		if _, exists := dateColumnMap[dateStr]; !exists {
			colNum := len(dateColumnMap) + 2
			dateColumnMap[dateStr] = colNum
			colName, err := excelize.ColumnNumberToName(colNum)
			if err != nil {
				log.Fatal(err)
			}
			f.SetCellValue("Sheet1", fmt.Sprintf("%s1", colName), dateStr)
		}

		colName, err := excelize.ColumnNumberToName(dateColumnMap[dateStr])
		if err != nil {
			log.Fatal(err)
		}
		f.SetCellValue("Sheet1", fmt.Sprintf("%s%d", colName, supplierRow[record.SupplierName]), record.BroughtMilk)
	}

	filename := time.Now().Format("2006-01-02") + ".xlsx"
	if err := f.SaveAs(filename); err != nil {
		log.Fatal(err)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Excel file generated successfully!"))
}

// package main

// import (
// 	"database/sql"
// 	"encoding/json"
// 	"fmt"
// 	"log"
// 	"net/http"
// 	"os"
// 	"time"

// 	"github.com/joho/godotenv"
// 	_ "github.com/lib/pq"
// 	"github.com/xuri/excelize/v2"
// )

// type Record struct {
// 	SupplierName    string
// 	TransactionDate time.Time
// 	BroughtMilk     float64
// }

// type DateRange struct {
// 	StartDate string `json:"start_date"`
// 	EndDate   string `json:"end_date"`
// }

// func main() {
// 	// load the .env file
// 	err := godotenv.Load()
// 	if err != nil {
// 		log.Fatal("Error loading .env file")
// 	}
// 	http.HandleFunc("/insert", insertHandler)
// 	log.Fatal(http.ListenAndServe(":8080", nil))
// }

// func insertHandler(w http.ResponseWriter, r *http.Request) {
// 	if r.Method != http.MethodPost {
// 		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	var dateRange DateRange
// 	err := json.NewDecoder(r.Body).Decode(&dateRange)
// 	if err != nil {
// 		http.Error(w, "Invalid request body", http.StatusBadRequest)
// 		return
// 	}

// 	// connStr := "host=DATABASE_HOST port=DATABASE_PORT user=DATABASE_USER dbname=DATABASE_NAME password=DATABASE_PASSWORD sslmode=disable"
// 	// redo the connection string with the env variables
// 	connStr := fmt.Sprintf("host=%s port=%s user=%s dbname=%s password=%s sslmode=disable", os.Getenv("DATABASE_HOST"), os.Getenv("DATABASE_PORT"), os.Getenv("DATABASE_USER"), os.Getenv("DATABASE_NAME"), os.Getenv("DATABASE_PASSWORD"))
// 	db, err := sql.Open("postgres", connStr)
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	query := fmt.Sprintf(`WITH DateSeries AS (
// 		SELECT generate_series(
// 		  '%s'::date,
// 		  '%s'::date,
// 		  '1 day'::interval
// 		) AS transaction_date
// 	  ),
// 	  AllFarmerDates AS (
// 		SELECT DISTINCT t.sno, ds.transaction_date
// 		FROM transactions t
// 		CROSS JOIN DateSeries ds
// 	  )
// 	  SELECT afd.sno,
// 			 afd.transaction_date,
// 			 COALESCE(t.total_qty, 0) AS TotalQty
// 	  FROM AllFarmerDates afd
// 	  LEFT JOIN transactions t
// 	  ON afd.sno = t.sno AND afd.transaction_date = DATE(t.transaction_date)
// 	  ORDER BY afd.sno, afd.transaction_date;
// 	  `, dateRange.StartDate, dateRange.EndDate)

// 	rows, err := db.Query(query)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	defer rows.Close()

// 	var records []Record

// 	for rows.Next() {
// 		var r Record
// 		if err := rows.Scan(&r.SupplierName, &r.TransactionDate, &r.BroughtMilk); err != nil {
// 			log.Fatal(err)
// 		}
// 		records = append(records, r)
// 		log.Println(r)
// 	}

// 	if err := rows.Err(); err != nil {
// 		log.Fatal(err)
// 	}

// 	f := excelize.NewFile()
// 	f.SetCellValue("Sheet1", "A1", "Supplier Name")

// 	supplierRow := make(map[string]int)
// 	dateColumnMap := make(map[string]int)

// 	for _, record := range records {
// 		if _, exists := supplierRow[record.SupplierName]; !exists {
// 			rowNum := len(supplierRow) + 2
// 			supplierRow[record.SupplierName] = rowNum
// 			f.SetCellValue("Sheet1", fmt.Sprintf("A%d", rowNum), record.SupplierName)
// 		}

// 		dateStr := record.TransactionDate.Format("2006-01-02")
// 		if _, exists := dateColumnMap[dateStr]; !exists {
// 			colNum := len(dateColumnMap) + 2
// 			dateColumnMap[dateStr] = colNum
// 			colName, err := excelize.ColumnNumberToName(colNum)
// 			if err != nil {
// 				log.Fatal(err)
// 			}
// 			f.SetCellValue("Sheet1", fmt.Sprintf("%s1", colName), dateStr)
// 		}

// 		colName, err := excelize.ColumnNumberToName(dateColumnMap[dateStr])
// 		if err != nil {
// 			log.Fatal(err)
// 		}
// 		f.SetCellValue("Sheet1", fmt.Sprintf("%s%d", colName, supplierRow[record.SupplierName]), record.BroughtMilk)
// 	}

// 	// Write the Excel file to a buffer
// 	buf, err := f.WriteToBuffer()
// 	if err != nil {
// 		http.Error(w, "Failed to generate Excel file", http.StatusInternalServerError)
// 		return
// 	}

// 	// Set the necessary headers to indicate that you're returning an Excel file
// 	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
// 	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", time.Now().Format("2006-01-02")+".xlsx"))

// 	// Write the buffer contents to the HTTP response
// 	w.Write(buf.Bytes())
// }
