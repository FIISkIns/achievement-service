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
	CourseId   string `json:"courseId"`
	CourseName string `json:"courseName"`
	CourseUrl  string `json:"courseUrl"`
}

func getJsonData(w http.ResponseWriter, url string, response interface{}) bool {
	failureResponse := "Failed communication with " + url + "\n"
	resp, err := http.Get(url)
	if err != nil {
		http.Error(w, failureResponse+err.Error(), http.StatusInternalServerError)
		return false
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, failureResponse+"Error reading response: "+err.Error(), http.StatusInternalServerError)
		return false
	}
	if resp.StatusCode != http.StatusOK {
		http.Error(w, failureResponse+"Response: "+string(body), http.StatusInternalServerError)
		return false
	}
	err = json.Unmarshal(body, &response)
	if err != nil {
		http.Error(w, "Failed to unmarshal data from "+url+": "+err.Error(), http.StatusInternalServerError)
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
			if item.Progress != "unresolved" {
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
	success := getJsonData(w, config.StatsServiceUrl+"/"+userId, &statsItem)
	if !success {
		return
	}

	var progressItems []ProgressItem
	success = getJsonData(w, config.CourseProgressServiceUrl+"/"+userId, &progressItems)
	if !success {
		return
	}

	var coursesInfo []CourseInfo
	success = getJsonData(w, config.CourseManagerServiceUrl+"/courses", &coursesInfo)
	if !success {
		return
	}

	achievements := make([]Achievement, 0)
	var achievement Achievement

	var totalTasksCompleted int
	for _, courseInfo := range coursesInfo {
		var achievementsInfo []AchievementInfo
		success = getJsonData(w, courseInfo.CourseUrl+"/achievements", &achievementsInfo)
		if !success {
			return
		}

		firstTaskCompletion, secondTaskCompletion, tasksCompleted := getProgressOnCourse(courseInfo.CourseId, progressItems)
		totalTasksCompleted += tasksCompleted
		for _, achievementInfo := range achievementsInfo {
			achievement.Title = achievementInfo.Title
			achievement.Description = achievementInfo.Description
			achievement.Icon = config.Adress + ":" + strconv.Itoa(config.Port) + "/static/" + courseInfo.CourseId + "/" + achievementInfo.Icon
			if achievementInfo.Type == "starter" {
				achievement.Completion = firstTaskCompletion
			} else if achievementInfo.Type == "completion" {
				achievement.Completion = secondTaskCompletion
			} else {
				//Error
				achievement.Completion = 0
			}
			achievements = append(achievements, achievement)
		}
	}

	for _, achievementInfo := range localAchievementsInfo {
		achievement.Title = achievementInfo.Title
		achievement.Description = achievementInfo.Description
		achievement.Icon = config.Adress + ":" + strconv.Itoa(config.Port) + "/static/" + achievementInfo.Icon
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
		achievements = append(achievements, achievement)
	}

	jsonData, err := json.Marshal(achievements)
	if err != nil {
		http.Error(w, "JSON error: failed to marshal stats", 500)
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
	type AchievementsInfoRaw struct {
		AchievementInfo `yaml:",inline"`
	}

	var info []AchievementsInfoRaw
	err := loadYaml("achievements.yml", &info)
	if err != nil {
		log.Panicln("While loading course info", err)
	}

	var achievementInfo AchievementInfo
	for _, achievement := range info {
		achievementInfo.Title = achievement.Title
		achievementInfo.Description = achievement.Description
		achievementInfo.Icon = achievement.Icon
		achievementInfo.Type = achievement.Type
		localAchievementsInfo = append(localAchievementsInfo, achievementInfo)
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
		if courseInfo.CourseId == params["course"] {
			newURL = courseInfo.CourseUrl + "/static/" + params["filepath"]
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

func main() {
	initConfig()
	loadAchievementsInfo()
	router := httptreemux.New()
	router.GET("/achievements/:user", getAchievementHandler)
	router.GET("/static/*filepath", getIconHandler)
	router.GET("/static/:course/*filepath", redirectHandler)
	err := http.ListenAndServe(":"+strconv.Itoa(config.Port), router)
	if err != nil {
		log.Fatal(err)
	}
}
