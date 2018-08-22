package main

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"

	"errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rds"
	_ "github.com/lib/pq"
	"github.com/nu7hatch/gouuid"
)

type DBParams struct {
	Dbname                  string
	Instanceid              string
	Masterusername          string
	Masterpassword          string
	Securitygroupid         string
	Allocatedstorage        int64
	Autominorversionupgrade bool
	Dbinstanceclass         string
	Dbparametergroupname    string
	Dbsubnetgroupname       string
	Multiaz                 bool
	Storageencrypted        bool
	Storagetype             string
	Iops                    int64
	Endpoint                string
}

func main() {
	uri := os.Getenv("BROKER_DB")
	db, err := sql.Open("postgres", uri)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	defer db.Close()

	// setup the database (or modify it as necessary)
	buf, err := ioutil.ReadFile("create.sql")
	if err != nil {
		log.Fatalf("Unable to read create.sql: %s\n", err)
	}
	_, err = db.Exec(string(buf))
	if err != nil {
		log.Fatal("Unable to create database: %s\n", err)
	}

	provision_micro, _ := strconv.Atoi(os.Getenv("PROVISION_MICRO"))
	provision_small, _ := strconv.Atoi(os.Getenv("PROVISION_SMALL"))
	provision_medium, _ := strconv.Atoi(os.Getenv("PROVISION_MEDIUM"))
	provision_large, _ := strconv.Atoi(os.Getenv("PROVISION_LARGE"))

	if need("micro", provision_micro) {
		record(provision_hobby(), "micro")
	}

	if need("small", provision_small) {
		record(provision("small"), "small")
	}

	if need("medium", provision_medium) {
		record(provision("medium"), "medium")
	}

	if need("large", provision_large) {
		record(provision("large"), "large")
	}

	insertEndpoints()
}

func record(dbparams DBParams, plan string) {
	uri := os.Getenv("BROKER_DB")
	db, err := sql.Open("postgres", uri)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	defer db.Close()
	var newname string
	err = db.QueryRow("INSERT INTO provision(name,plan,claimed,masterpass,masteruser,endpoint) VALUES($1,$2,$3,$4,$5,$6) returning name;", dbparams.Dbname, plan, "no", dbparams.Masterpassword, dbparams.Masterusername, dbparams.Endpoint).Scan(&newname)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	fmt.Println(newname)
	err = db.Close()

}

func provision_hobby() DBParams {
	uri := os.Getenv("HOBBY_DB")
	hobby_admin := os.Getenv("HOBBY_ADMIN")
	dbparams := new(DBParams)

	//Generate Unique DB Name
	dbnameuuid, _ := uuid.NewV4()
	dbparams.Dbname = os.Getenv("NAME_PREFIX") + strings.Split(dbnameuuid.String(), "-")[0]
	fmt.Println("DB Name: " + dbparams.Dbname)
	dbparams.Instanceid = dbparams.Dbname

	//Generate Unique User ID
	usernameuuid, _ := uuid.NewV4()
	dbparams.Masterusername = "u" + strings.Split(usernameuuid.String(), "-")[0]
	fmt.Println("User Name: " + dbparams.Masterusername)

	//Generate Unique Password
	passworduuid, _ := uuid.NewV4()
	dbparams.Masterpassword = strings.Split(passworduuid.String(), "-")[0] + strings.Split(passworduuid.String(), "-")[1]
	fmt.Println("Password: " + dbparams.Masterpassword)

	db, err := sql.Open("postgres", uri)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	defer db.Close()

	_, dberr := db.Exec("CREATE USER " + dbparams.Masterusername + " WITH PASSWORD '" + dbparams.Masterpassword + "' NOINHERIT")
	fmt.Println("creating user")
	if dberr != nil {
		fmt.Println(dberr)
		os.Exit(2)
	}

	_, dberr = db.Exec("GRANT " + dbparams.Masterusername + " TO " + hobby_admin)
	fmt.Println("granting permission")
	if dberr != nil {
		fmt.Println(dberr)
		os.Exit(2)
	}

	_, dberr = db.Exec("CREATE DATABASE " + dbparams.Dbname + " OWNER " + dbparams.Masterusername)
	fmt.Println("granting permission")
	if dberr != nil {
		fmt.Println(dberr)
		os.Exit(2)
	}

	dbparams.Endpoint = os.Getenv("HOBBY_ENDPOINT") + "/" + dbparams.Dbname
	return *dbparams
}

func provision(plan string) DBParams {

	dbparams := new(DBParams)

	dbnameuuid, _ := uuid.NewV4()
	dbparams.Dbname = os.Getenv("NAME_PREFIX") + strings.Split(dbnameuuid.String(), "-")[0]
	fmt.Println(dbparams.Dbname)
	dbparams.Instanceid = dbparams.Dbname

	usernameuuid, _ := uuid.NewV4()
	dbparams.Masterusername = "u" + strings.Split(usernameuuid.String(), "-")[0]
	fmt.Println(dbparams.Masterusername)

	passworduuid, _ := uuid.NewV4()
	dbparams.Masterpassword = strings.Split(passworduuid.String(), "-")[0] + strings.Split(passworduuid.String(), "-")[1]
	fmt.Println(dbparams.Masterpassword)

	dbparams.Securitygroupid = os.Getenv("RDS_SECURITY_GROUP")

	switch plan {
	case "small":
		dbparams.Allocatedstorage = int64(20)
		dbparams.Autominorversionupgrade = true
		dbparams.Dbinstanceclass = os.Getenv("SMALL_INSTANCE_TYPE")
		dbparams.Dbparametergroupname = "rds-postgres-small"
		dbparams.Dbsubnetgroupname = "rds-postgres-subnet-group"
		dbparams.Multiaz = false
		dbparams.Storageencrypted = false
		dbparams.Storagetype = "gp2"
		dbparams.Iops = int64(0)
	case "medium":
		dbparams.Allocatedstorage = int64(50)
		dbparams.Autominorversionupgrade = false
		dbparams.Dbinstanceclass = os.Getenv("MEDIUM_INSTANCE_TYPE")
		dbparams.Dbparametergroupname = "rds-postgres-medium"
		dbparams.Dbsubnetgroupname = "rds-postgres-subnet-group"
		dbparams.Multiaz = false
		dbparams.Storageencrypted = false
		dbparams.Storagetype = "gp2"
		dbparams.Iops = int64(0)
	case "large":
		dbparams.Allocatedstorage = int64(100)
		dbparams.Autominorversionupgrade = false
		dbparams.Dbinstanceclass = os.Getenv("LARGE_INSTANCE_TYPE")
		dbparams.Dbparametergroupname = "rds-postgres-large"
		dbparams.Dbsubnetgroupname = "rds-postgres-subnet-group"
		dbparams.Multiaz = true
		dbparams.Storageencrypted = true
		dbparams.Storagetype = "io1"
		dbparams.Iops = int64(1000)
	}
	svc := rds.New(session.New(&aws.Config{
		Region: aws.String(os.Getenv("REGION")),
	}))

	params := &rds.CreateDBInstanceInput{
		DBInstanceClass:         aws.String(dbparams.Dbinstanceclass), // Required
		DBInstanceIdentifier:    aws.String(dbparams.Instanceid),      // Required
		Engine:                  aws.String("postgres"),               // Required
		AllocatedStorage:        aws.Int64(dbparams.Allocatedstorage),
		AutoMinorVersionUpgrade: aws.Bool(dbparams.Autominorversionupgrade),
		//AvailabilityZone:        aws.String("String"),
		//BackupRetentionPeriod:   aws.Int64(1),
		//CharacterSetName:        aws.String("String"),
		//CopyTagsToSnapshot:      aws.Bool(true),
		//DBClusterIdentifier:     aws.String("String"),
		DBName:               aws.String(dbparams.Dbname),
		DBParameterGroupName: aws.String(dbparams.Dbparametergroupname),
		//DBSecurityGroups: []*string{
		//	aws.String("String"), // Required
		//	// More values...
		//},
		DBSubnetGroupName: aws.String(dbparams.Dbsubnetgroupname),
		//Domain:             aws.String("String"),
		//DomainIAMRoleName:  aws.String("String"),
		EngineVersion: aws.String("9.5.2"),
		Iops:          aws.Int64(dbparams.Iops),
		//KmsKeyId:           aws.String("String"),
		//LicenseModel:       aws.String("String"),
		MasterUserPassword: aws.String(dbparams.Masterpassword),
		MasterUsername:     aws.String(dbparams.Masterusername),
		//MonitoringInterval: aws.Int64(1),
		//MonitoringRoleArn:  aws.String("String"),
		MultiAZ: aws.Bool(dbparams.Multiaz),
		//OptionGroupName:    aws.String("String"),
		Port: aws.Int64(5432),
		//PreferredBackupWindow:      aws.String("String"),
		//PreferredMaintenanceWindow: aws.String("String"),
		//PromotionTier:              aws.Int64(1),
		PubliclyAccessible: aws.Bool(false),
		StorageEncrypted:   aws.Bool(dbparams.Storageencrypted),
		StorageType:        aws.String(dbparams.Storagetype),
		Tags: []*rds.Tag{
			{ // Required
				Key:   aws.String("Name"),
				Value: aws.String(dbparams.Dbname),
			},
			{ // Required
				Key:   aws.String("billingcode"),
				Value: aws.String("pre-provisioned"),
			},
		},
		//TdeCredentialArn:      aws.String("String"),
		//TdeCredentialPassword: aws.String("String"),
		VpcSecurityGroupIds: []*string{
			aws.String(dbparams.Securitygroupid), // Required
		},
	}
	resp, err := svc.CreateDBInstance(params)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(2)
	}

	fmt.Println(resp)
	return *dbparams
}

func need(plan string, minimum int) bool {
	uri := os.Getenv("BROKER_DB")
	db, err := sql.Open("postgres", uri)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	defer db.Close()

	var unclaimedcount int
	err = db.QueryRow("SELECT count(*) as unclaimedcount from provision where plan='" + plan + "' and claimed='no'").Scan(&unclaimedcount)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	fmt.Println(unclaimedcount)
	if unclaimedcount < minimum {
		return true
	}
	return false
}

func isAvailable(name string) bool {
	var toreturn bool
	region := os.Getenv("REGION")

	svc := rds.New(session.New(&aws.Config{
		Region: aws.String(region),
	}))

	rparams := &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(name),
		MaxRecords:           aws.Int64(20),
	}
	rresp, rerr := svc.DescribeDBInstances(rparams)
	if rerr != nil {
		fmt.Println(rerr)
	}
	//      fmt.Println(rresp)
	fmt.Println("Checking to see if available...")
	fmt.Println(name + " Status: " + *rresp.DBInstances[0].DBInstanceStatus)
	status := *rresp.DBInstances[0].DBInstanceStatus
	if status == "available" {
		toreturn = true
	}
	if status != "available" {
		toreturn = false
	}
	return toreturn
}

func insertEndpoints() {
	uri := os.Getenv("BROKER_DB")
	db, err := sql.Open("postgres", uri)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	defer db.Close()

	rows, err := db.Query("select name from provision where endpoint='' OR masteruser=''")
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}

	fmt.Println("Looking for endpoints")

	for rows.Next() {
		var name string
		err = rows.Scan(&name)
		if err != nil {
			fmt.Println(err)
			os.Exit(2)
		}

		fmt.Println("attempting add endpoint for " + name)
		if isAvailable(name) {
			endpoint, username, eerr := getEndpoint(name)
			if eerr != nil {
				fmt.Println(err)
				os.Exit(2)
			}
			addEndpoint(name, endpoint, username)
		}
	}
}

func addEndpoint(name string, endpoint string, username string) {
	uri := os.Getenv("BROKER_DB")
	db, err := sql.Open("postgres", uri)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	defer db.Close()

	stmt, err := db.Prepare("UPDATE provision SET endpoint=$1,masteruser=$2 WHERE name=$3")
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	_, err = stmt.Exec(endpoint, username, name)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
}

func getEndpoint(name string) (endpoint string, username string, err error) {
	region := os.Getenv("REGION")
	svc := rds.New(session.New(&aws.Config{
		Region: aws.String(region),
	}))
	params := &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(name),
		MaxRecords:           aws.Int64(20),
	}
	resp, err := svc.DescribeDBInstances(params)
	if err != nil {
		fmt.Println(err)
		err = errors.New("failed to get instance information: " + name)
		return "", endpoint, err
	}

	endpoint = *resp.DBInstances[0].Endpoint.Address + ":" + strconv.FormatInt(*resp.DBInstances[0].Endpoint.Port, 10) + "/" + name
	username = *resp.DBInstances[0].MasterUsername
	return endpoint, username, nil
}
