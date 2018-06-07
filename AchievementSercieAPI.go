package main

import (
	"database/sql"
	"encoding/json"
	"github.com/julienschmidt/httprouter"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"sort"
	"fmt"
)

var database *sql.DB

type ProgressItem struct {
	CourseId string `json:"courseId"`
	TaskId   string `json:"taskId"`
	Progress string `json:"progress"`
}

type StatsItem struct {
	StartedCourses   int    `json:"startedCourses"`
	CompletedCourses int    `json:"completedCourses"`
	LastLoggedIn     string `json:"lastLoggedIn"`
	TimeSpent        int    `json:"timeSpent"`
	LongestStreak    int    `json:"longestStreak"`
}

type Achievement struct {
	Icon        string `json:"icon"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Completion  int    `json:"completion"`
}

func getStats(userId string) (*StatsItem, error) {
	resp, err := http.Get(config.StatsServiceUrl + "/" + userId)
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var statsItem StatsItem
	err = json.Unmarshal(body, &statsItem)
	return &statsItem, err
}

func getProgress(userId string) ([]ProgressItem, error) {
	resp, err := http.Get(config.CourseProgressServiceUrl + "/" + userId)
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var progressItems []ProgressItem
	err = json.Unmarshal(body, &progressItems)
	sort.Slice(progressItems, func(i, j int) bool { return progressItems[i].CourseId < progressItems[j].CourseId })
	return progressItems, err
}

func getAchievementHandler(w http.ResponseWriter, _ *http.Request, ps httprouter.Params) {
	userId := ps.ByName("user")
	statsItem, err := getStats(userId)
	if err != nil {
		//handle error
		fmt.Println("stats")
	}

	achievements := make([]Achievement, 0)
	achievement := Achievement{Icon: "link", Title: "Spread the word!", Description: "Share a course on Facebook", Completion: 0}
	achievements = append(achievements, achievement)

	var completion int
	if statsItem.LongestStreak <= 10 {
		completion = statsItem.LongestStreak * 10
	} else {
		completion = 100
	}
	achievement = Achievement{Icon: "link", Title: "Learning marathon!", Description: "Get a streak of 10 days", Completion: completion}
	achievements = append(achievements, achievement)

	progressItems, err := getProgress(userId)
	if err != nil {
		//handle error
		fmt.Println("progress")
	}
	var totalNoOfTasks int
	var noOfCourseTasks int
	var tasksCompleted int
	var courseId string
	var courseStarted bool
	sort.Slice(progressItems, func(i, j int) bool { return progressItems[i].CourseId < progressItems[j].CourseId })
	for i, item := range progressItems {
		if courseId != item.CourseId || i == len(progressItems)-1 {
			if i > 0 {
				if courseStarted {
					completion = 100
				} else {
					completion = 0
				}
				achievement = Achievement{Icon: "link", Title: "started course: " + courseId, Description: "Start any task in the course", Completion: completion}
				achievements = append(achievements, achievement)

				completion = (tasksCompleted * 10) / noOfCourseTasks * 10
				achievement = Achievement{Icon: "link", Title: "completed course: " + courseId, Description: "Complete all course tasks", Completion: completion}
				achievements = append(achievements, achievement)

				courseStarted = false
				tasksCompleted = 0
				noOfCourseTasks = 0
			}
			courseId = item.CourseId
		}
		if item.Progress != "unresolved" {
			courseStarted = true
		}
		if item.Progress == "completed" {
			tasksCompleted++
		}
		noOfCourseTasks++
		totalNoOfTasks++
	}

	if totalNoOfTasks <= 10 {
		completion = totalNoOfTasks * 10
	} else {
		completion = 100
	}
	achievement = Achievement{Icon: "link", Title: "Seriously committed!", Description: "Complete 10 tasks", Completion: completion}
	achievements = append(achievements, achievement)

	jsonData, err := json.Marshal(achievements)
	if err != nil {
		http.Error(w, "JSON error: failed to marshal stats", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)
}

func main() {
	initConfig()
	defer database.Close()
	router := httprouter.New()
	router.GET("/:user", getAchievementHandler)
	err := http.ListenAndServe(":"+strconv.Itoa(config.Port), router)
	if err != nil {
		log.Fatal(err)
	}
}
