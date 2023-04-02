// based on looking at the auto-increment id, is not affected when entries are deleted because new entires's ids are greater than old deleted ids. ids are not repeated.

package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
	"strings"
	_ "github.com/go-sql-driver/mysql"
)

var (
	table            string
	monitorFrequency int
)

type DBoutput struct {
	output string `json:"output"`
}

func seq(first int, jumpNum int, last int) (sequence []int) {
	for i := first; i <= last; i = i + jumpNum {
		sequence = append(sequence, i)
	}
	return sequence
}

func getMaxID(db *sql.DB, tableName string) (maxID int) {

	query := "SELECT max(id) from " + tableName
	err := db.QueryRow(query).Scan(&maxID)

	if err != nil {
		panic(err.Error())
	}

	return maxID

}

func dumpDB(db *sql.DB, fromID int, toID int, column string, table string) (output string) {

	Query := fmt.Sprintf("SELECT %s FROM %s where `id`>%d and `id`<=%d", column, table, fromID, toID)
	fmt.Println(Query)
	results, err := db.Query(Query)

	if err != nil {
		panic(err.Error()) // proper error handling instead of panic in your app
	}

	for results.Next() {
		var dbOutput DBoutput

		err = results.Scan(&dbOutput.output)

		if err != nil {
			panic(err.Error())
		}

		output += dbOutput.output + "\\n"

	}

	return output

}

func callRabbitMQSend(message string, queue string) {
	// TO DO : change path of rabbit mq script
	rabbitMQSendScript := "/home/ec2-user/rabbitmq/send.go"

	// call bash script that does "grep -vf nucleiBlacklist and send to rabbitmq" or do directly using bash -c "command to grep -v and send to rabbitmq here."

	//	cmd := exec.Command("bash", "-c", command)

	cmd := exec.Command("go", "run", rabbitMQSendScript, "-priority", "99", "-message", message, "-queue", queue)

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(fmt.Sprint(err) + ": " + string(output))
		//TODO : call function to send mail here
	}
	fmt.Println(string(output))
}

func doJob(db *sql.DB, table string, oldCount int) (newCount int) {
	var dumpFrom int
	var dumpTo int

	newCount = getMaxID(db, table)

	if oldCount != 0 && newCount != oldCount {
		switch table {
		case "subdomains":
			sequence := seq(oldCount, 5000, newCount)
			fmt.Println("sequence : ", sequence)
			for s, _ := range sequence {
				if s+1 < len(sequence) {
					fmt.Println(sequence[s], sequence[s+1])
					dumpFrom, dumpTo = sequence[s], sequence[s+1]
				} else {
					fmt.Println(sequence[s], newCount)
					dumpFrom, dumpTo = sequence[s], newCount
				}
				subdomains2send := dumpDB(db, dumpFrom, dumpTo, "subdomain", "subdomains")
				fmt.Println(subdomains2send)
				fmt.Println("-------------------")
				// sending directly to nuclei for now
				if subdomains2send != "" {
					// message := fmt.Sprintf("{\"domains\":\"%s\"}", subdomains2send)
					// callRabbitMQSend(message, "nuclei")

					WrapperScript := "/home/ec2-user/reResolve_vps_files/wrapper4nuclei_new_subs.sh"
					cmd := exec.Command("bash", WrapperScript, subdomains2send)

					output, err := cmd.CombinedOutput()
					if err != nil {
						fmt.Println(fmt.Sprint(err) + ": " + string(output))
					}
					fmt.Println(string(output))				
				}
			}
			fmt.Println("-------------------")
		// case "httpx":
		// 	seq := seq(oldCount, 50, newCount)
		// 	for _, s := range seq {
		// 		fmt.Println("subdomain", s)
		// 		//callRabbitMQSend(s, "nuclei")
		// 	}
		case "nuclei":
			fmt.Println(oldCount, newCount)
			nucleiOutput := dumpDB(db, oldCount, newCount, "output", "nuclei")
			fmt.Println(nucleiOutput)
			var notify_bot string
			for _, result := range strings.Split(nucleiOutput, "\\n") {
				fmt.Println("result",result)
				if strings.HasPrefix(result, "critical:") {
					notify_bot = "nuclei_critical"
				} else if strings.HasPrefix(result, "high:") {
					notify_bot = "nuclei_high"
				} else if strings.HasPrefix(result, "medium:") {
					notify_bot = "nuclei_medium"
				} else {
					notify_bot = "nuclei_low"
				}
				fmt.Println("nucleioutput",nucleiOutput)
				fmt.Println(notify_bot)
			        //send mail
				command := fmt.Sprintf("printf '%s' | notify -id %s", result ,notify_bot)
				fmt.Println("command",command)
				cmd := exec.Command("bash", "-c", command)
				output, err := cmd.CombinedOutput()
				if err != nil {
					fmt.Println(fmt.Sprint(err) + ": " + string(output))
					//TODO : call function to send mail here
				}
			}

		}
	}
	//fmt.Println(oldCount, newCount)
	return newCount
}

func main() {

	flag.StringVar(&table, "table", "", "table to monitor")
	flag.IntVar(&monitorFrequency, "monitorFrequency", 600, "time to query DB on")

	flag.Parse()

	if table == "" {
		log.Fatalln("table name to monitor not passed, see -h")
	}

	mysql_database_name := "automation"

	mysql_username := os.Getenv("mysql_username")
	mysql_password := os.Getenv("mysql_password")
	mysql_host := os.Getenv("mysql_host")
	mysql_port := os.Getenv("mysql_port")

	connectionURL := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", mysql_username, mysql_password, mysql_host, mysql_port, mysql_database_name)

	db, err := sql.Open("mysql", connectionURL)

	if err != nil {
		panic(err.Error())
	}

	// defer the close till after the main function has finished
	// executing
	defer db.Close()

	// perform a db.Query

	oldCount := 0
	fmt.Println("Monitoring table : ", table)
	for true {

		fmt.Println("calling")
		newCount := doJob(db, table, oldCount)

		oldCount = newCount

		time.Sleep(time.Duration(monitorFrequency) * time.Second)

	}

}

