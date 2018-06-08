package main

import (
	"database/sql"
	"encoding/json"
	"github.com/dimfeld/httptreemux"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"sort"
	"fmt"
	"path/filepath"
	"path"
	"gopkg.in/yaml.v2"
	"net/url"
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

func getCoursesInfo() ([]CourseInfo, error) {
	resp, err := http.Get(config.CourseManagerServiceUrl + "/courses")
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var coursesInfo []CourseInfo
	err = json.Unmarshal(body, &coursesInfo)
	sort.Slice(coursesInfo, func(i, j int) bool { return coursesInfo[i].CourseId < coursesInfo[j].CourseId })
	return coursesInfo, err
}

func getAchievementsInfo(courseInfo CourseInfo) ([]AchievementInfo, error) {
	resp, err := http.Get(courseInfo.CourseUrl + "/achievements")
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var achievementsInfo []AchievementInfo
	err = json.Unmarshal(body, &achievementsInfo)
	return achievementsInfo, err
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
	statsItem, err := getStats(userId)
	if err != nil {
		//handle error
		fmt.Println("stats", err.Error())
	}

	progressItems, err := getProgress(userId)
	if err != nil {
		//handle error
		fmt.Println("progress", err.Error())
	}

	coursesInfo, err := getCoursesInfo()
	if err != nil {
		//handle error
		fmt.Println("courses", err.Error())
	}

	achievements := make([]Achievement, 0)
	var achievement Achievement

	var totalTasksCompleted int
	for _, courseInfo := range coursesInfo {
		achievementsInfo, err := getAchievementsInfo(courseInfo)
		if err != nil {
			log.Fatal(err)
		}
		firstTaskCompletion, secondTaskCompletion, tasksCompleted := getProgressOnCourse(courseInfo.CourseId, progressItems)
		totalTasksCompleted += tasksCompleted
		for _, achievementInfo := range achievementsInfo {
			achievement.Title = achievementInfo.Title
			achievement.Description = achievementInfo.Description
			achievement.Icon = courseInfo.CourseUrl + "/static/" + achievementInfo.Icon
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

func redirect(w http.ResponseWriter, r *http.Request, newPath string, statusCode int) {
	newURL := url.URL{
		Path:     newPath,
	}
	http.Redirect(w, r, newURL.String(), statusCode)
}

func redirectHandler(w http.ResponseWriter, r *http.Request, params map[string]string) {
	coursesInfo, err := getCoursesInfo()
	if err != nil {
		//handle error
		fmt.Println("courses", err.Error())
	}
	var newPath string
	for _, courseInfo := range coursesInfo{
		if courseInfo.CourseId == params["course"] {
			newPath = courseInfo.CourseUrl + "/static/" + params["filepath"]
			break
		}

	}
	redirect(w, r, newPath, http.StatusOK)
}

func main() {
	initConfig()
	loadAchievementsInfo()
	router := httptreemux.New()
	router.GET("/:user", getAchievementHandler)
	router.GET("/static/*filepath", getIconHandler)
	router.GET("/static/:course/*filepath",redirectHandler)
	err := http.ListenAndServe(":"+strconv.Itoa(config.Port), router)
	if err != nil {
		log.Fatal(err)
	}
}
