package discord

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/imdario/mergo"
	"github.com/l1ghthouse/northstar-bootstrap/src/mod"
	"github.com/l1ghthouse/northstar-bootstrap/src/nsserver"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"io"
	"log"
)

type Notifier struct {
	discordClient                     *discordgo.Session
	reportChannel                     string
	RebalancedLTSRankingMongoDBString string
}

func NewNotifier(discordClient *discordgo.Session, reportChannel string, rebalancedLTSRankingMongoDBString string) *Notifier {
	if reportChannel == "" && rebalancedLTSRankingMongoDBString == "" {
		return nil
	}
	return &Notifier{
		discordClient:                     discordClient,
		reportChannel:                     reportChannel,
		RebalancedLTSRankingMongoDBString: rebalancedLTSRankingMongoDBString,
	}
}

func (d *Notifier) NotifyServer(server *nsserver.NSServer, message string) {
	if d.reportChannel != "" {
		sendMessage(d.discordClient, d.reportChannel, fmt.Sprintf("Server %s:\n", server.Name)+message)
	}
}

func (d *Notifier) NotifyAndAttachServerData(server *nsserver.NSServer, message string, filename string, file *bytes.Buffer) {
	if d.reportChannel != "" {
		if file != nil {
			if d.RebalancedLTSRankingMongoDBString != "" {
				buffer := bytes.NewBuffer(file.Bytes())
				go d.processRebalancedLTSLogs(*server, d.RebalancedLTSRankingMongoDBString, buffer)
			}
			sendComplexMessage(d.discordClient, d.reportChannel, fmt.Sprintf("Server %s:\n", server.Name)+message, []*discordgo.File{{
				Name:        filename,
				ContentType: "application/octet-stream",
				Reader:      file,
			}})
		} else {
			d.NotifyServer(server, message)
		}
	}
}

var rebalancedString = []byte("[LTSRebalanceData]")

type LTSRebalanceLogID struct {
	UID     string
	MatchID string
	Round   int
}

type LTSRebalanceLogStruct struct {
	UID                          string  `json:"uid" bson:"uid"`
	Round                        int     `json:"round" bson:"round"`
	MatchID                      string  `json:"matchID" bson:"matchID"`
	MatchTimestamp               int     `json:"matchTimestamp" bson:"matchTimestamp"`
	Ranked                       bool    `json:"ranked" bson:"ranked"`
	Name                         string  `json:"name" bson:"name"`
	Rebalance                    bool    `json:"rebalance" bson:"rebalance"`
	PerfectKits                  bool    `json:"perfectKits" bson:"perfectKits"`
	MapName                      string  `json:"mapName" bson:"mapName"`
	Team                         int     `json:"team" bson:"team"`
	Result                       string  `json:"result" bson:"result"`
	RoundDuration                float32 `json:"roundDuration" bson:"roundDuration"`
	Titan                        string  `json:"titan" bson:"titan"`
	Kit1                         string  `json:"kit1" bson:"kit1"`
	Kit2                         string  `json:"kit2" bson:"kit2"`
	Core1                        string  `json:"core1" bson:"core1"`
	Core2                        string  `json:"core2" bson:"core2"`
	Core3                        string  `json:"core3" bson:"core3"`
	DamageDealtBlockedTarget     int     `json:"damageDealtBlockedTarget" bson:"damageDealtBlockedTarget"`
	DamageTakenBlockedTarget     int     `json:"damageTakenBlockedTarget" bson:"damageTakenBlockedTarget"`
	DamageDealtCloseAlly         int     `json:"damageDealtCloseAlly" bson:"damageDealtCloseAlly"`
	DamageDealtCloseEnemy        int     `json:"damageDealtCloseEnemy" bson:"damageDealtCloseEnemy"`
	DamageDealt                  int     `json:"damageDealt" bson:"damageDealt"`
	DamageDealtShields           int     `json:"damageDealtShields" bson:"damageDealtShields"`
	DamageDealtTempShields       int     `json:"damageDealtTempShields" bson:"damageDealtTempShields"`
	DamageDealtAuto              int     `json:"damageDealtAuto" bson:"damageDealtAuto"`
	DamageDealtPilot             int     `json:"damageDealtPilot" bson:"damageDealtPilot"`
	DamageDealtBlocked           int     `json:"damageDealtBlocked" bson:"damageDealtBlocked"`
	DamageDealtSelf              int     `json:"damageDealtSelf" bson:"damageDealtSelf"`
	CritRateDealt                float32 `json:"critRateDealt" bson:"critRateDealt"`
	DamageTaken                  int     `json:"damageTaken" bson:"damageTaken"`
	DamageTakenShields           int     `json:"damageTakenShields" bson:"damageTakenShields"`
	DamageTakenTempShields       int     `json:"damageTakenTempShields" bson:"damageTakenTempShields"`
	DamageTakenAuto              int     `json:"damageTakenAuto" bson:"damageTakenAuto"`
	DamageTakenBlocked           int     `json:"damageTakenBlocked" bson:"damageTakenBlocked"`
	CritRateTaken                float32 `json:"critRateTaken" bson:"critRateTaken"`
	Kills                        int     `json:"kills" bson:"kills"`
	KillsPilot                   int     `json:"killsPilot" bson:"killsPilot"`
	Terminations                 int     `json:"terminations" bson:"terminations"`
	TerminationDamage            int     `json:"terminationDamage" bson:"terminationDamage"`
	CoreFracEarned               float32 `json:"coreFracEarned" bson:"coreFracEarned"`
	CoresUsed                    int     `json:"coresUsed" bson:"coresUsed"`
	BatteriesPicked              int     `json:"batteriesPicked" bson:"batteriesPicked"`
	BatteriesToSelf              int     `json:"batteriesToSelf" bson:"batteriesToSelf"`
	BatteriesToAlly              int     `json:"batteriesToAlly" bson:"batteriesToAlly"`
	BatteriesToAllyPilot         int     `json:"batteriesToAllyPilot" bson:"batteriesToAllyPilot"`
	ShieldsGained                int     `json:"shieldsGained" bson:"shieldsGained"`
	TempShieldsGained            int     `json:"tempShieldsGained" bson:"tempShieldsGained"`
	HealthWasted                 int     `json:"healthWasted" bson:"healthWasted"`
	ShieldsWasted                int     `json:"shieldsWasted" bson:"shieldsWasted"`
	TimeAsTitan                  float32 `json:"timeAsTitan" bson:"timeAsTitan"`
	TimeDeathTitan               float32 `json:"timeDeathTitan" bson:"timeDeathTitan"`
	TimeAsPilot                  float32 `json:"timeAsPilot" bson:"timeAsPilot"`
	TimeDeathPilot               float32 `json:"timeDeathPilot" bson:"timeDeathPilot"`
	Ejection                     bool    `json:"ejection" bson:"ejection"`
	AvgDistanceToCenterAllies    float32 `json:"avgDistanceToCenterAllies" bson:"avgDistanceToCenterAllies"`
	AvgDistanceToCenterEnemies   float32 `json:"avgDistanceToCenterEnemies" bson:"avgDistanceToCenterEnemies"`
	AvgDistanceBetweenCenters    float32 `json:"avgDistanceBetweenCenters" bson:"avgDistanceBetweenCenters"`
	AvgDistanceToBatteries       float32 `json:"avgDistanceToBatteries" bson:"avgDistanceToBatteries"`
	AvgDistanceToCloseBattery    float32 `json:"avgDistanceToCloseBattery" bson:"avgDistanceToCloseBattery"`
	AvgDistanceOnCritDealt       float32 `json:"avgDistanceOnCritDealt" bson:"avgDistanceOnCritDealt"`
	AvgDistanceOnNonCritDealt    float32 `json:"avgDistanceOnNonCritDealt" bson:"avgDistanceOnNonCritDealt"`
	AvgDistanceToAllies          float32 `json:"avgDistanceToAllies" bson:"avgDistanceToAllies"`
	AvgDistanceToCloseAlly       float32 `json:"avgDistanceToCloseAlly" bson:"avgDistanceToCloseAlly"`
	AvgDistanceToEnemies         float32 `json:"avgDistanceToEnemies" bson:"avgDistanceToEnemies"`
	AvgDistanceToCloseEnemy      float32 `json:"avgDistanceToCloseEnemy" bson:"avgDistanceToCloseEnemy"`
	AvgDistanceToAlliesPilot     float32 `json:"avgDistanceToAlliesPilot" bson:"avgDistanceToAlliesPilot"`
	AvgDistanceToCloseAllyPilot  float32 `json:"avgDistanceToCloseAllyPilot" bson:"avgDistanceToCloseAllyPilot"`
	AvgDistanceToEnemiesPilot    float32 `json:"avgDistanceToEnemiesPilot" bson:"avgDistanceToEnemiesPilot"`
	AvgDistanceToCloseEnemyPilot float32 `json:"avgDistanceToCloseEnemyPilot" bson:"avgDistanceToCloseEnemyPilot"`
	DistanceTravelled            float32 `json:"distanceTravelled" bson:"distanceTravelled"`
	DistanceTravelledPilot       float32 `json:"distanceTravelledPilot" bson:"distanceTravelledPilot"`
	Version                      string  `json:"version" bson:"version"`

	// Additional Metadata fields
	ServerName   string `json:"serverName" bson:"serverName"`
	IsPreRelease bool   `json:"isPreRelease" bson:"isPreRelease"`
}

func extractData(zipArchive *bytes.Buffer) map[LTSRebalanceLogID]LTSRebalanceLogStruct {
	//https://stackoverflow.com/questions/50539118/golang-unzip-response-body for in memory zip files

	archive, err := zip.NewReader(bytes.NewReader(zipArchive.Bytes()), int64(zipArchive.Len()))
	if err != nil {
		log.Println(err)
		return nil
	}

	if len(archive.File) != 1 {
		log.Println("zip file contains more than one file")
		return nil
	}

	fileInArchive, err := archive.File[0].Open()
	if err != nil {
		log.Println("error opening file in archive", err)
		return nil
	}
	defer func(fileInArchive io.ReadCloser) {
		err := fileInArchive.Close()
		if err != nil {
			log.Println("error closing file in archive", err)
		}
	}(fileInArchive)

	buf := bytes.NewBuffer(nil)
	_, err = io.Copy(buf, fileInArchive)
	if err != nil {
		log.Println("error copying file in archive to buffer", err)
		return nil
	}
	var logLines [][]byte
	for _, line := range bytes.Split(buf.Bytes(), []byte("\n")) {
		idx := bytes.Index(line, rebalancedString)
		if idx != -1 {
			logLines = append(logLines, line[idx+len(rebalancedString)+1:])
		}
	}

	var dict = make(map[LTSRebalanceLogID]LTSRebalanceLogStruct)

	for _, line := range logLines {
		var logStruct LTSRebalanceLogStruct
		err := json.Unmarshal(line, &logStruct)
		if err != nil {
			log.Println("error unmarshalling ranking json", err)
		}

		if logStruct.UID == "" || logStruct.MatchID == "" || logStruct.Round == 0 {
			log.Printf("Error: one of the following is an empty value: UID, MatchID, Round. Skipping line: %s", line)
			continue
		}

		LTSRebalanceLogID := LTSRebalanceLogID{
			UID:     logStruct.UID,
			MatchID: logStruct.MatchID,
			Round:   logStruct.Round,
		}
		if val, ok := dict[LTSRebalanceLogID]; ok {
			if err := mergo.Merge(&logStruct, val); err != nil {
				log.Println("error merging logStruct", err)
			}
		}
		dict[LTSRebalanceLogID] = logStruct
	}

	return dict
}

func (d *Notifier) processRebalancedLTSLogs(server nsserver.NSServer, mongodbConnection string, zipArchive *bytes.Buffer) {

	rankingData := extractData(zipArchive)

	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(mongodbConnection))
	if err != nil {
		log.Println("Error connecting to mongodb: ", err)
		return
	}

	defer func() {
		if err = client.Disconnect(context.Background()); err != nil {
			log.Println("Error disconnecting from mongodb: ", err)
		}
	}()

	rankingDataSlice := make([]interface{}, 0, len(rankingData))

	for _, value := range rankingData {
		value.ServerName = server.Name
		v, ok := server.ModOptions[mod.RebalancedLtsModTest].(bool)
		if ok {
			value.IsPreRelease = v
		}
		rankingDataSlice = append(rankingDataSlice, value)
	}

	_, err = client.Database("ranking").Collection("ranking").InsertMany(context.Background(), rankingDataSlice, options.InsertMany().SetOrdered(false))
	if err != nil {
		log.Println("error inserting many", err)
		d.NotifyServer(&server, fmt.Sprintf("Error inserting ranking data: %v", err.Error()))
		return
	}

	d.NotifyServer(&server, "Successfully inserted ranking data")

}
