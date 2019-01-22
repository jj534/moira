package checker

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/moira-alert/moira"
	"github.com/moira-alert/moira/database"
	"github.com/moira-alert/moira/metric_source"
	"github.com/moira-alert/moira/metric_source/local"
	"github.com/moira-alert/moira/metrics/graphite"
	"github.com/moira-alert/moira/mock/moira-alert"
	"github.com/op/go-logging"
	. "github.com/smartystreets/goconvey/convey"
)

func TestInitTriggerChecker(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	logger, _ := logging.GetLogger("Test")
	config := &Config{}
	dataBase := mock_moira_alert.NewMockDatabase(mockCtrl)
	localSource := local.Create(dataBase)
	triggerID := "superId"
	defer mockCtrl.Finish()

	Convey("Test errors", t, func() {
		Convey("Get trigger error", func() {
			getTriggerError := fmt.Errorf("Oppps! Can't read trigger")
			dataBase.EXPECT().GetTrigger(triggerID).Return(moira.Trigger{}, getTriggerError)
			_, err := MakeTriggerChecker(triggerID, dataBase, logger, config, metricSource.CreateMetricSourceProvider(localSource, nil), &graphite.CheckerMetrics{})
			So(err, ShouldBeError)
			So(err, ShouldResemble, getTriggerError)
		})

		Convey("No trigger error", func() {
			dataBase.EXPECT().GetTrigger(triggerID).Return(moira.Trigger{}, database.ErrNil)
			_, err := MakeTriggerChecker(triggerID, dataBase, logger, config, metricSource.CreateMetricSourceProvider(localSource, nil), &graphite.CheckerMetrics{})
			So(err, ShouldBeError)
			So(err, ShouldResemble, ErrTriggerNotExists)
		})

		Convey("Get lastCheck error", func() {
			readLastCheckError := fmt.Errorf("Oppps! Can't read last check")
			dataBase.EXPECT().GetTrigger(triggerID).Return(moira.Trigger{TriggerType: moira.RisingTrigger}, nil)
			dataBase.EXPECT().GetTriggerLastCheck(triggerID).Return(moira.CheckData{}, readLastCheckError)
			_, err := MakeTriggerChecker(triggerID, dataBase, logger, config, metricSource.CreateMetricSourceProvider(localSource, nil), &graphite.CheckerMetrics{})
			So(err, ShouldBeError)
			So(err, ShouldResemble, readLastCheckError)
		})
	})

	var warnValue float64 = 10000
	var errorValue float64 = 100000
	var ttl int64 = 900
	var value float64
	ttlStateOk := OK
	ttlStateNoData := NODATA

	trigger := moira.Trigger{
		ID:          "d39b8510-b2f4-448c-b881-824658c58128",
		Name:        "Time",
		Targets:     []string{"aliasByNode(Metric.*.time, 1)"},
		WarnValue:   &warnValue,
		ErrorValue:  &errorValue,
		TriggerType: moira.RisingTrigger,
		Tags:        []string{"tag1", "tag2"},
		TTLState:    &ttlStateOk,
		Patterns:    []string{"Egais.elasticsearch.*.*.jvm.gc.collection.time"},
		TTL:         ttl,
	}

	lastCheck := moira.CheckData{
		Timestamp: 1502694487,
		State:     OK,
		Score:     0,
		Metrics: map[string]moira.MetricState{
			"1": {
				Timestamp:      1502694427,
				State:          OK,
				Suppressed:     false,
				Value:          &value,
				EventTimestamp: 1501680428,
			},
			"2": {
				Timestamp:      1502694427,
				State:          OK,
				Suppressed:     false,
				Value:          &value,
				EventTimestamp: 1501679827,
			},
			"3": {
				Timestamp:      1502694427,
				State:          OK,
				Suppressed:     false,
				Value:          &value,
				EventTimestamp: 1501679887,
			},
		},
	}

	Convey("Test trigger checker with lastCheck", t, func() {
		dataBase.EXPECT().GetTrigger(triggerID).Return(trigger, nil)
		dataBase.EXPECT().GetTriggerLastCheck(triggerID).Return(lastCheck, nil)
		actual, err := MakeTriggerChecker(triggerID, dataBase, logger, config, metricSource.CreateMetricSourceProvider(localSource, nil), &graphite.CheckerMetrics{})
		So(err, ShouldBeNil)

		expected := TriggerChecker{
			TriggerID: triggerID,
			Database:  dataBase,
			Config:    config,
			Source:    localSource,
			Logger:    logger,
			trigger:   &trigger,
			ttl:       trigger.TTL,
			ttlState:  *trigger.TTLState,
			lastCheck: &lastCheck,
			From:      lastCheck.Timestamp - ttl,
			Until:     actual.Until,
		}
		So(*actual, ShouldResemble, expected)
	})

	Convey("Test trigger checker without lastCheck", t, func() {
		dataBase.EXPECT().GetTrigger(triggerID).Return(trigger, nil)
		dataBase.EXPECT().GetTriggerLastCheck(triggerID).Return(moira.CheckData{}, database.ErrNil)
		actual, err := MakeTriggerChecker(triggerID, dataBase, logger, config, metricSource.CreateMetricSourceProvider(localSource, nil), &graphite.CheckerMetrics{})
		So(err, ShouldBeNil)

		expected := TriggerChecker{
			TriggerID: triggerID,
			Database:  dataBase,
			Config:    config,
			Source:    localSource,
			Logger:    logger,
			trigger:   &trigger,
			ttl:       trigger.TTL,
			ttlState:  *trigger.TTLState,
			lastCheck: &moira.CheckData{
				Metrics:   make(map[string]moira.MetricState),
				State:     OK,
				Timestamp: actual.Until - 3600,
			},
			From:  actual.Until - 3600 - ttl,
			Until: actual.Until,
		}
		So(*actual, ShouldResemble, expected)
	})

	trigger.TTL = 0
	trigger.TTLState = nil

	Convey("Test trigger checker without lastCheck and ttl", t, func() {
		dataBase.EXPECT().GetTrigger(triggerID).Return(trigger, nil)
		dataBase.EXPECT().GetTriggerLastCheck(triggerID).Return(moira.CheckData{}, database.ErrNil)
		actual, err := MakeTriggerChecker(triggerID, dataBase, logger, config, metricSource.CreateMetricSourceProvider(localSource, nil), &graphite.CheckerMetrics{})
		So(err, ShouldBeNil)

		expected := TriggerChecker{
			TriggerID: triggerID,
			Database:  dataBase,
			Config:    config,
			Source:    localSource,
			Logger:    logger,
			trigger:   &trigger,
			ttl:       0,
			ttlState:  ttlStateNoData,
			lastCheck: &moira.CheckData{
				Metrics:   make(map[string]moira.MetricState),
				State:     OK,
				Timestamp: actual.Until - 3600,
			},
			From:  actual.Until - 3600 - 600,
			Until: actual.Until,
		}
		So(*actual, ShouldResemble, expected)
	})

	Convey("Test trigger checker with lastCheck and without ttl", t, func() {
		dataBase.EXPECT().GetTrigger(triggerID).Return(trigger, nil)
		dataBase.EXPECT().GetTriggerLastCheck(triggerID).Return(lastCheck, nil)
		actual, err := MakeTriggerChecker(triggerID, dataBase, logger, config, metricSource.CreateMetricSourceProvider(localSource, nil), &graphite.CheckerMetrics{})
		So(err, ShouldBeNil)

		expected := TriggerChecker{
			TriggerID: triggerID,
			Database:  dataBase,
			Config:    config,
			Source:    localSource,
			Logger:    logger,
			trigger:   &trigger,
			ttl:       0,
			ttlState:  ttlStateNoData,
			lastCheck: &lastCheck,
			From:      lastCheck.Timestamp - 600,
			Until:     actual.Until,
		}
		So(*actual, ShouldResemble, expected)
	})
}
