// CLI utility for managing gitlab users
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const GitlabURL = "https://git.domain.com/api/v4/"

type GitlabUsers []struct {
	ID              int       `json:"id"`
	Name            string    `json:"name"`
	Username        string    `json:"username"`
	State           string    `json:"state"`
	CreatedAt       time.Time `json:"created_at"`
	PublicEmail     string    `json:"public_email"`
	LastSignInAt    time.Time `json:"last_sign_in_at"`
	ConfirmedAt     time.Time `json:"confirmed_at"`
	LastActivityOn  string    `json:"last_activity_on"`
	Email           string    `json:"email"`
	CurrentSignInAt time.Time `json:"current_sign_in_at"`
	External        bool      `json:"external"`
	IsAdmin         bool      `json:"is_admin"`
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func (s GitlabUsers) Len() int {
	return len(s)
}

func (s GitlabUsers) Less(i, j int) bool {
	return s[i].ID < s[j].ID
}

func (s GitlabUsers) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// getUsers gets the list of users from Gitlab.  It takes a string representing an API token and a boolean which tells it
// whether or not to limit the search to active users.  It returns a GitlabUsers struct populated with users
func getUsers(t string, a bool) GitlabUsers {
	a_str := strconv.FormatBool(a)
	response, httpErr := http.Get(GitlabURL + "users?active=" + a_str + "&external=false&order_by=id&page=1&per_page=100&skip_ldap=false&sort=desc&with_custom_attributes=false&private_token=" + t)
	check(httpErr)
	defer response.Body.Close()
	data, responseErr := ioutil.ReadAll(response.Body)
	check(responseErr)
	var jsonResponse GitlabUsers
	jsonErr := json.Unmarshal(data, &jsonResponse)
	check(jsonErr)
	currentPage, err := strconv.Atoi(response.Header["X-Page"][0])
	check(err)
	totalPages, err := strconv.Atoi(response.Header["X-Total-Pages"][0])
	check(err)
	// Retrieve additional pages if needed
	for ; currentPage < totalPages; currentPage++ {
		nextPage := strconv.Itoa(currentPage + 1)
		response, httpErr := http.Get(GitlabURL + "users?active=" + a_str + "&external=false&order_by=id&page=" + nextPage + "&per_page=100&skip_ldap=false&sort=desc&with_custom_attributes=false&private_token=" + t)
		check(httpErr)
		defer response.Body.Close()
		data, responseErr := ioutil.ReadAll(response.Body)
		check(responseErr)
		var tmpJsonResponse GitlabUsers
		jsonErr := json.Unmarshal(data, &tmpJsonResponse)
		check(jsonErr)
		jsonResponse = append(jsonResponse, tmpJsonResponse...)
	}
	return jsonResponse
}

// displayAllUsers print out all users
func displayAllUsers(u GitlabUsers) {
	for _, v := range u {
		fmt.Println("-----------------------")
		fmt.Println("ID :", v.ID)
		fmt.Println("Name :", v.Name)
	}
}

// searchUser takes some text to search and parses through the GitlabUsers struct
// it checks for matches on Name, Username or Email
func searchUsers(s string, u GitlabUsers) {
	type foundUser struct {
		Name     string
		ID       int
		Username string
		Email    string
	}
	var foundUsers []foundUser
	for i, _ := range u {
		if strings.Contains(u[i].Name, s) || strings.Contains(u[i].Username, s) || strings.Contains(u[i].Email, s) {
			foundUsers = append(foundUsers, foundUser{
				Name:     u[i].Name,
				ID:       u[i].ID,
				Username: u[i].Username,
				Email:    u[i].Email,
			})
		}
	}

	if len(foundUsers) > 0 {
		//fmt.Printf("ID"\t"Name"\t"Username"\t"Email"\n)
		for i, _ := range foundUsers {
			fmt.Printf("%v \t%v \t%v \t%v\n", foundUsers[i].ID, foundUsers[i].Name, foundUsers[i].Username, foundUsers[i].Email)
		}
	} else {
		fmt.Println("No users found")
	}
}

// createUser takes input from the console and creates an account in Gitlab
// The mimimum information to create an account is Name, Username and Email
func createUser(t string) {
	// Get input from console
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Name: ")
	inputName, _ := reader.ReadString('\n')
	inputName = strings.Replace(inputName, "\n", "", -1)

	fmt.Print("Email: ")
	inputEmail, _ := reader.ReadString('\n')
	inputEmail = strings.Replace(inputEmail, "\n", "", -1)

	fmt.Print("Username: ")
	inputUserName, _ := reader.ReadString('\n')
	inputUserName = strings.Replace(inputUserName, "\n", "", -1)

	// check that inputUserName has no whitespace
	var isAlpha = regexp.MustCompile(`^[A-Za-z]+$`)
	if !isAlpha.MatchString(inputUserName) {
		log.Fatalf("error: %v is not a valid username", inputUserName)
	}

	// check that inputEmail is valid
	// taken from https://www.alexedwards.net/blog/validation-snippets-for-go#required-inputs
	var rxEmail = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")
	if len(inputEmail) > 254 || !rxEmail.MatchString(inputEmail) {
		log.Fatalf("error: %v is not a valid email address", inputEmail)
	}

	// Create JSON payload
	payload := map[string]interface{}{
		"email":          inputEmail,
		"name":           inputName,
		"username":       inputUserName,
		"reset_password": "true",
	}
	bytesRepresentation, err := json.Marshal(payload)
	check(err)

	resp, postErr := http.Post(GitlabURL + "users?private_token=" + t, "application/json", bytes.NewBuffer(bytesRepresentation))
	check(postErr)

	fmt.Println(resp.Status)
}

// blockUser takes an API token and a user ID to block
func blockUser(t string, u string) {
	resp, postErr := http.Post(GitlabURL + "users/" + u + "/block?private_token=" + t, "application/x-www-form-urlencoded", io.Reader(nil))
	check(postErr)

	// handle respose from block call
	switch resp.Status {
	case "201 Created":
		fmt.Printf("UserID %v has been blocked\n", u)
		fmt.Println(resp.Status)
	case "404 Not Found":
		fmt.Printf("ERROR: Unable to find UserID %v\n", u)
		fmt.Println(resp.Status)
	case "403 Forbidden":
		fmt.Printf("ERROR: UserID %v is already blocked\n", u)
		fmt.Println(resp.Status)
	default:
		fmt.Printf("Error: something went wrong while trying to block UserID %v\n", u)
		fmt.Println(resp.Status)
	}
}

func main() {
	var active bool
	var searchText string
	var token string
	var create bool
	var bu string
	flag.StringVar(&token, "t", "YOUR_ACCESS_TOKEN_HERE", "Gitlab access token")
	flag.StringVar(&searchText, "s", "", "search userbase")
	flag.BoolVar(&create, "c", false, "create user")
	flag.BoolVar(&active, "a", true, "limit search to active users")
	flag.StringVar(&bu, "b", "", "user id to block")
	flag.Parse()

	if searchText != "" {
		users := getUsers(token, active)
		sort.Sort(users)
		searchUsers(searchText, users)
	} else if bu != "" {
		blockUser(token, bu)
	} else if create {
		createUser(token)
	} else {
		users := getUsers(token, active)
		sort.Sort(users)
		displayAllUsers(users)
	}
}
