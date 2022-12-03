package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/forms"
	"github.com/pocketbase/pocketbase/models"
)

type MeetingResponse struct {
	MeetingId  string       `json:"meeting_id"`
	Host       string       `json:"host"`
	Devotional DevoResponse `json:"devotional"`
}

type DevoResponse struct {
	Video     string   `json:"video"`
	Verses    string   `json:"verses"`
	Questions []string `json:"questions"`
}

func insert_record(app *pocketbase.PocketBase, collection_name string, insert_map map[string]any) error {
	collection, err := app.Dao().FindCollectionByNameOrId(collection_name)
	if err != nil {
		return err
	}
	record := models.NewRecord(collection)
	form := forms.NewRecordUpsert(app, record)
	form.LoadData(insert_map)
	if err := form.Submit(); err != nil {
		return err
	}
	return nil
}

func GetRandomDevo(app *pocketbase.PocketBase) (*models.Record, error) {
	record, err := app.Dao().FindRecordById("devos", "el7vgh7ef7lr7yu")
	if err != nil {
		fmt.Println("DEFAULT DEVO IS MISSING!")
		return nil, err
	}
	fmt.Println("WARNING: YOU'RE NOT RANDOMLY CHOOSING A DEVO.")
	return record, nil
}

func main() {
	app := pocketbase.New()
	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		e.Router.AddRoute(echo.Route{
			Method: http.MethodPut,
			Path:   "/m", //CREATE A MEETING
			Handler: func(c echo.Context) error {
				devoRecord, _ := GetRandomDevo(app)
				authRecord, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)
				meetup_record := map[string]any{
					"devotional": devoRecord.Id,
					"host":       authRecord.Id,
				}
				err := insert_record(app, "meeting", meetup_record)
				if err != nil {
					return c.String(http.StatusInternalServerError, err.Error())
				}
				fmt.Println("We inserted a meeting")
				//TODO DO SOMETHING BETTER HERE!!! REDIRECT TO JOIN!
				return c.String(http.StatusOK, "Inserted a custom thing")
				//return c.Redirect(http.StatusCreated, )
			},
			Middlewares: []echo.MiddlewareFunc{
				apis.RequireRecordAuth(),
				apis.ActivityLogger(app),
			},
		})
		e.Router.AddRoute(echo.Route{
			Method: http.MethodGet,
			Path:   "/m/:id", //JOIN A MEETING
			Handler: func(c echo.Context) error {
				//1. get the user id
				authRecord, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)
				//2. check the meeting id exists
				meetingRecord, err := app.Dao().FindRecordById("meeting", c.PathParam("id"))
				if err != nil {
					fmt.Println("INVALID MEETING ID")
					return c.String(http.StatusNotFound, "Couldn't find meeting id.")
				}
				//3. TODO check if the user is already in the meeting

				//4. if the user is not add the user to the meeting
				userMeetupMap := map[string]any{
					"meeting":     meetingRecord.Id,
					"participant": authRecord.Id,
				}
				err = insert_record(app, "usermeeting", userMeetupMap)
				if err != nil {
					return c.String(http.StatusInternalServerError, err.Error())
				}
				//5. get the host username of the meeting
				hostRecord, err := app.Dao().FindRecordById("users", meetingRecord.GetString("host"))
				if err != nil {
					fmt.Println("Somehow we don't have a host for a meeting???")
				}
				meetingHostname := hostRecord.Username()
				//6. get the devotional record
				devotionalRecord, err := app.Dao().FindRecordById("devos", meetingRecord.GetString("devotional"))
				if err != nil {
					fmt.Println("The devotional associated with the meeting was missing.")
				}
				//7. Get the questions associated with the devotional
				questionsRecords, _ := app.Dao().FindRecordsByExpr("questions", dbx.HashExp{"devo": devotionalRecord.Id})
				questions := make([]string, 0)
				for _, v := range questionsRecords {
					questions = append(questions, v.GetString("question"))
				}
				//8. Build the response
				dr := &DevoResponse{
					Video:     devotionalRecord.GetString("video"),
					Verses:    devotionalRecord.GetString("verses"),
					Questions: questions,
				}
				mr := &MeetingResponse{
					Devotional: *dr,
					Host:       meetingHostname,
					MeetingId:  meetingRecord.Id,
				}
				return c.JSON(http.StatusOK, mr)
			},
			Middlewares: []echo.MiddlewareFunc{
				apis.RequireRecordAuth(),
				apis.ActivityLogger(app),
			},
		})
		return nil
	})
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
