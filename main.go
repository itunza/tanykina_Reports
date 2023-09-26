package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/xuri/excelize/v2"
)

var allowedAgents = []string{
	"0704422997@spaceai.io",
	"254712789533@spaceai.io",
	"254759298692@spaceai.io",
	"abel.kipkemei@spaceai.io",
	"abraham.kogo@spaceai.io",
	"brian.kiplimo@spaceai.io",
	"chemaget.justus@spaceai.io",
	"edwin.tuwei@spaceai.io",
	"jesphat.kiprotich@spaceai.io",
	"meli.abel@spaceai.io",
	"sammy.koech@spaceai.io",
	"vitalis.leonard@spaceai.io",
	// ... add more agent names as needed
}

type Record struct {
	SupplierName    string
	TransactionDate time.Time
	BroughtMilk     float64
}
type AgentRecord struct {
	AgentName       string
	TransactionDate time.Time
	CollectedMilk   float64
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
	http.HandleFunc("/agents", agentsHandler)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
func agentsHandler(w http.ResponseWriter, r *http.Request) {
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

	query := fmt.Sprintf(`
    WITH DateSeries AS (
        SELECT generate_series(
            '%s'::date, 
            '%s'::date, 
            '1 day'::interval
        ) AS transaction_date
    ),
    AllAgentDates AS (
        SELECT DISTINCT t.agent, ds.transaction_date
        FROM transactions t
        CROSS JOIN DateSeries ds
    ),
    GroupedTransactions AS (
        SELECT agent, DATE(transaction_date) as transaction_date, SUM(total_qty) as total_qty
        FROM transactions
        GROUP BY agent, DATE(transaction_date)
    )
    SELECT aad.agent, 
           aad.transaction_date,
           COALESCE(gt.total_qty, 0) AS TotalQty
    FROM AllAgentDates aad
    LEFT JOIN GroupedTransactions gt
    ON aad.agent = gt.agent AND aad.transaction_date = gt.transaction_date
    ORDER BY aad.agent, aad.transaction_date;
    `, dateRange.StartDate, dateRange.EndDate)

	rows, err := db.Query(query)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	var records []AgentRecord
	dateTotals := make(map[string]float64)

	for rows.Next() {
		var r AgentRecord
		if err := rows.Scan(&r.AgentName, &r.TransactionDate, &r.CollectedMilk); err != nil {
			log.Fatal(err)
		}
		if isIncludedAgent(r.AgentName) { // Check if the agent should be included
			records = append(records, r)
			dateStr := r.TransactionDate.Format("2006-01-02")
			dateTotals[dateStr] += r.CollectedMilk
		}
	}

	f := excelize.NewFile()
	f.SetCellValue("Sheet1", "A1", "Agent Name")

	agentRow := make(map[string]int)
	dateColumnMap := make(map[string]int)

	for _, record := range records {
		if _, exists := agentRow[record.AgentName]; !exists {
			rowNum := len(agentRow) + 2
			agentRow[record.AgentName] = rowNum
			f.SetCellValue("Sheet1", fmt.Sprintf("A%d", rowNum), record.AgentName)
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
		f.SetCellValue("Sheet1", fmt.Sprintf("%s%d", colName, agentRow[record.AgentName]), record.CollectedMilk)
	}

	// Save the Excel data to a byte buffer and return it in the response
	buf := new(bytes.Buffer)
	_, err = f.WriteTo(buf)
	if err != nil {
		http.Error(w, "Failed to write to buffer", http.StatusInternalServerError)
		return
	}

	// Set the appropriate headers for downloading an Excel file
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=agent_report.xlsx")

	// Write the buffer contents to the HTTP response
	w.Write(buf.Bytes())
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
	query := fmt.Sprintf(`
	WITH DateSeries AS (
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
	),
	GroupedTransactions AS (
		SELECT sno, DATE(transaction_date) as transaction_date, SUM(total_qty) as total_qty
		FROM transactions
		GROUP BY sno, DATE(transaction_date)
	)
	SELECT afd.sno, 
			 afd.transaction_date,
			 COALESCE(gt.total_qty, 0) AS TotalQty
	FROM AllFarmerDates afd
	LEFT JOIN GroupedTransactions gt
	ON afd.sno = gt.sno AND afd.transaction_date = gt.transaction_date
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
		if isAllowedSupplier(r.SupplierName) {
			records = append(records, r)
		}
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
	buf := new(bytes.Buffer)
	_, err = f.WriteTo(buf)
	if err != nil {
		http.Error(w, "Failed to write to buffer", http.StatusInternalServerError)
		return
	}

	// Set the appropriate headers for downloading an Excel file
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)

	// Write the buffer contents to the HTTP response
	w.Write(buf.Bytes())
	// if err := f.SaveAs(filename); err != nil {
	// 	log.Fatal(err)
	// }

	// w.WriteHeader(http.StatusOK)
	// w.Write([]byte("Excel file generated successfully!"))
}

func isAllowedSupplier(supplierName string) bool {
	// List of allowed prefixes
	allowedPrefixes := []string{"sur", "sal", "kpk", "san"}
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(strings.ToLower(supplierName), prefix) {
			// Check if the prefix is more than 3 text characters
			i := 3
			for ; i < len(supplierName) && unicode.IsLetter(rune(supplierName[i])); i++ {
			}
			return i <= 3
		}
	}
	return false
}

func isIncludedAgent(agentName string) bool {
	for _, allowedAgent := range allowedAgents {
		if agentName == allowedAgent {
			return true
		}
	}
	return false
}
