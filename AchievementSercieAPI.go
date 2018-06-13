package main

import (
	"encoding/json"
	"github.com/dimfeld/httptreemux"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
)

type ProgressItem struct {
	CourseId string
	TaskId   string
	Progress string
}

type StatsItem struct {
	StartedCourses   int
	CompletedCourses int
	LastLoggedIn     string
	TimeSpent        int
	LongestStreak    int
}

type Achievement struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Completion  int    `json:"completion"`
}

type AchievementInfo struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Type        string `json:"type"`
}

var localAchievementsInfo = make([]AchievementInfo, 0)

type CourseInfo struct {
	Id   string
	Name string
	Url  string
}

func getJsonData(w http.ResponseWriter, url string, response interface{}) bool {
	failureResponse := "Failed communication with " + url + "\n"
	resp, err := http.Get(url)
	if err != nil {
		errorMessage := failureResponse+err.Error()
		log.Println(errorMessage)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		return false
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		errorMessage := failureResponse+"Error reading response: "+err.Error()
		log.Println(errorMessage)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		return false
	}
	if resp.StatusCode != http.StatusOK {
		errorMessage := failureResponse+"Response: "+string(body)
		log.Println(errorMessage)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		return false
	}
	err = json.Unmarshal(body, &response)
	if err != nil {
		errorMessage := "Failed to unmarshal data from "+url+": "+err.Error()
		log.Println(errorMessage)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		return false
	}
	return true
}

func getProgressOnCourse(courseId string, progress []ProgressItem) (int, int, int) {
	var noOfCourseTasks int
	var tasksCompleted int
	var courseStarted bool
	var firstTaskCompletion int
	var secondTaskCompletion int
	for _, item := range progress {
		if courseId == item.CourseId {
			if item.Progress != "not started" {
				courseStarted = true
			}
			if item.Progress == "completed" {
				tasksCompleted++
			}
			noOfCourseTasks++
		}
	}
	if courseStarted {
		firstTaskCompletion = 100
	} else {
		firstTaskCompletion = 0
	}

	secondTaskCompletion = (tasksCompleted * 10) / noOfCourseTasks * 10

	return firstTaskCompletion, secondTaskCompletion, tasksCompleted
}

func getAchievementHandler(w http.ResponseWriter, _ *http.Request, params map[string]string) {
	userId := params["user"]
	var statsItem StatsItem
	success := getJsonData(w, config.StatsServiceUrl+"/stats/"+userId, &statsItem)
	if !success {
		return
	}

	var progressItems []ProgressItem
	success = getJsonData(w, config.CourseProgressServiceUrl+"/progress/"+userId, &progressItems)
	if !success {
		return
	}

	var coursesInfo []CourseInfo
	success = getJsonData(w, config.CourseManagerServiceUrl+"/courses", &coursesInfo)
	if !success {
		return
	}

	achievements := make([]Achievement, (len(coursesInfo)*2)+len(localAchievementsInfo))

	var achievement Achievement
	var totalTasksCompleted int

	var index int
	for _, courseInfo := range coursesInfo {
		var achievementsInfo []AchievementInfo
		success = getJsonData(w, courseInfo.Url+"/achievements", &achievementsInfo)
		if !success {
			return
		}

		firstTaskCompletion, secondTaskCompletion, tasksCompleted := getProgressOnCourse(courseInfo.Id, progressItems)
		totalTasksCompleted += tasksCompleted
		for _, achievementInfo := range achievementsInfo {
			achievement.Title = achievementInfo.Title
			achievement.Description = achievementInfo.Description
			achievement.Icon = courseInfo.Id + "/" + achievementInfo.Icon
			if achievementInfo.Type == "starter" {
				achievement.Completion = firstTaskCompletion
			} else if achievementInfo.Type == "completion" {
				achievement.Completion = secondTaskCompletion
			} else {
				//Error
				achievement.Completion = 0
			}
			achievements[index] = achievement
			index++
		}
	}

	for _, achievementInfo := range localAchievementsInfo {
		achievement.Title = achievementInfo.Title
		achievement.Description = achievementInfo.Description
		achievement.Icon = achievementInfo.Icon
		switch achievementInfo.Type {
		case "start":
			if totalTasksCompleted != 0 {
				achievement.Completion = 100
			} else {
				achievement.Completion = 0
			}
		case "share":
			achievement.Completion = 0
		case "study":
			if totalTasksCompleted <= 10 {
				achievement.Completion = totalTasksCompleted * 10
			} else {
				achievement.Completion = 100
			}
		case "marathon":
			if statsItem.LongestStreak <= 10 {
				achievement.Completion = statsItem.LongestStreak * 10
			} else {
				achievement.Completion = 100
			}
		default:
			achievement.Completion = 0
		}
		achievements[index] = achievement
		index++
	}

	jsonData, err := json.Marshal(achievements)
	if err != nil {
		errorMessage := "JSON error (failed to marshal stats): "+err.Error()
		log.Println(errorMessage)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)
}

func getIconHandler(w http.ResponseWriter, r *http.Request, params map[string]string) {
	http.ServeFile(w, r, filepath.Join(config.Path, "resources", params["filepath"]))
	return
}

func loadYaml(filePath string, v interface{}) error {
	log.Println("Loading achievements file", filePath)

	data, err := ioutil.ReadFile(path.Join(config.Path, filePath))
	if err != nil {
		return err
	}

	return yaml.Unmarshal(data, v)
}

func loadAchievementsInfo() {
	err := loadYaml("achievements.yml", &localAchievementsInfo)
	if err != nil {
		log.Panicln("While loading course info", err)
	}
	log.Println("Achievements info loaded successfully.")
}

func redirectHandler(w http.ResponseWriter, r *http.Request, params map[string]string) {
	var coursesInfo []CourseInfo
	success := getJsonData(w, config.CourseManagerServiceUrl+"/courses", &coursesInfo)
	if !success {
		return
	}
	var courseFound bool
	var newURL string
	for _, courseInfo := range coursesInfo {
		if courseInfo.Id == params["course"] {
			newURL = courseInfo.Url + "/static/" + params["filepath"]
			courseFound = true
			break
		}
	}
	if courseFound {
		http.Redirect(w, r, newURL, http.StatusPermanentRedirect)
	} else {
		http.Error(w, "Specified course not found", http.StatusNotFound)
	}

}

func checkHealth(w http.ResponseWriter, url string) bool {
	resp, err := http.Get(url)
	if err != nil {
		errorMessage := "Failed to communicate with: " + url + "\nCause: " + err.Error()
		log.Println(errorMessage)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		return false
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		errorMessage := "Failed to read response from: " + url + "\nCause: " + err.Error()
		log.Println(errorMessage)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		return false
	}
	if resp.StatusCode != http.StatusOK {
		errorMessage := "Failed health check on: " + url + "\nResponse: " + string(body)
		log.Println(errorMessage)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		return false
	}
	return true
}

func healthCheckHandler(w http.ResponseWriter, _ *http.Request, _ map[string]string) {
	if success := checkHealth(w, config.CourseProgressServiceUrl+"/health"); !success {
		return
	}
	if success := checkHealth(w, config.CourseManagerServiceUrl+"/health"); !success {
		return
	}
	if success := checkHealth(w, config.StatsServiceUrl+"/health"); !success {
		return
	}
}

func main() {
	initConfig()
	loadAchievementsInfo()
	router := httptreemux.New()
	router.GET("/health", healthCheckHandler)
	router.GET("/achievements/:user", getAchievementHandler)
	router.GET("/static/*filepath", getIconHandler)
	router.GET("/static/:course/*filepath", redirectHandler)
	err := http.ListenAndServe(":"+strconv.Itoa(config.Port), router)
	if err != nil {
		log.Fatal(err)
	}
}
