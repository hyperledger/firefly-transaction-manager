// Copyright © 2024 Kaleido, Inc.
//
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metrics

import "context"

const metricsLabelStatus = "status"
const metricsLabelType = "type"

const mtrCounterReceiptCheckTotal = "receipt_check_total"
const mtrCounterReceiptCheckTotalDescription = "Number of receipt check happened grouped by receipt check status"
const mtrHistogramReceiptCheckDuration = "receipt_check_duration_seconds"
const mtrHistogramReceiptCheckDurationDescription = "Duration of individual receipt check grouped by receipt check status"

const mtrCounterConfirmationNotificationQueuedTotal = "confirmation_notification_queued_total"
const mtrCounterConfirmationNotificationQueuedTotalDescription = "Number of confirmation notification queued grouped by notification type"
const mtrHistogramConfirmationNotificationQueueingDuration = "confirmation_notification_queueing_delay_seconds"
const mtrHistogramConfirmationNotificationQueueingDurationDescription = "Duration of adding a notification into the queue grouped by notification type"

const mtrCounterConfirmationNotificationProcessedTotal = "confirmation_notification_processed_total"
const mtrCounterConfirmationNotificationProcessedTotalDescription = "Number of confirmation notification processed grouped by notification type"
const mtrHistogramConfirmationNotificationProcessDuration = "confirmation_notification_process_seconds"
const mtrHistogramConfirmationNotificationProcessDurationDescription = "Duration of processing a notification grouped by notification type"

const mtrCounterConfirmationBlockHashProcessedTotal = "confirmation_block_hash_processed_total"
const mtrCounterConfirmationBlockHashProcessedTotalDescription = "Number of block hashes processed by confirmation manager"
const mtrHistogramConfirmationBlockHashProcessDuration = "confirmation_block_hash_process_duration_seconds"
const mtrHistogramConfirmationBlockHashProcessDurationDescription = "Duration of processing a block hash received by confirmation manager"

const mtrCounterConfirmedTotal = "confirmation_processed_total"
const mtrCounterConfirmedTotalDescription = "Number of transactions confirmed"
const mtrHistogramConfirmDuration = "confirmation_block_hash_process_duration_seconds"
const mtrHistogramConfirmDurationDescription = "Duration of confirm a transaction"

type EventMetricsEmitter interface {
	EventStreamMetricsEmitter
	ConfirmationMetricsEmitter
}

type EventStreamMetricsEmitter interface {
}

type ConfirmationMetricsEmitter interface {
	RecordConfirmationMetrics(ctx context.Context, durationInSeconds float64)
	RecordBlockHashProcessMetrics(ctx context.Context, durationInSeconds float64)
	RecordNotificationProcessMetrics(ctx context.Context, notificationType string, durationInSeconds float64)
	RecordNotificationQueueingMetrics(ctx context.Context, notificationType string, durationInSeconds float64)
	ReceiptCheckerMetricsEmitter
}

type ReceiptCheckerMetricsEmitter interface {
	RecordReceiptCheckMetrics(ctx context.Context, status string, durationInSeconds float64)
}

func (mm *metricsManager) RecordReceiptCheckMetrics(ctx context.Context, status string, durationInSeconds float64) {
	mm.eventsMetricsManager.IncCounterMetricWithLabels(ctx, mtrCounterReceiptCheckTotal, map[string]string{metricsLabelStatus: status}, nil)
	mm.eventsMetricsManager.ObserveHistogramMetricWithLabels(ctx, mtrHistogramReceiptCheckDuration, durationInSeconds, map[string]string{metricsLabelStatus: status}, nil)
}

func (mm *metricsManager) RecordBlockHashProcessMetrics(ctx context.Context, durationInSeconds float64) {
	mm.eventsMetricsManager.IncCounterMetric(ctx, mtrCounterConfirmationBlockHashProcessedTotal, nil)
	mm.eventsMetricsManager.ObserveHistogramMetric(ctx, mtrHistogramConfirmationBlockHashProcessDuration, durationInSeconds, nil)
}
func (mm *metricsManager) RecordNotificationProcessMetrics(ctx context.Context, notificationType string, durationInSeconds float64) {
	mm.eventsMetricsManager.IncCounterMetricWithLabels(ctx, mtrCounterConfirmationNotificationProcessedTotal, map[string]string{metricsLabelType: notificationType}, nil)
	mm.eventsMetricsManager.ObserveHistogramMetricWithLabels(ctx, mtrHistogramConfirmationNotificationProcessDuration, durationInSeconds, map[string]string{metricsLabelType: notificationType}, nil)
}

func (mm *metricsManager) RecordNotificationQueueingMetrics(ctx context.Context, notificationType string, durationInSeconds float64) {
	mm.eventsMetricsManager.IncCounterMetricWithLabels(ctx, mtrCounterConfirmationNotificationQueuedTotal, map[string]string{metricsLabelType: notificationType}, nil)
	mm.eventsMetricsManager.ObserveHistogramMetricWithLabels(ctx, mtrHistogramConfirmationNotificationQueueingDuration, durationInSeconds, map[string]string{metricsLabelType: notificationType}, nil)
}

func (mm *metricsManager) RecordConfirmationMetrics(ctx context.Context, durationInSeconds float64) {
	mm.eventsMetricsManager.IncCounterMetric(ctx, mtrCounterConfirmedTotal, nil)
	mm.eventsMetricsManager.ObserveHistogramMetric(ctx, mtrHistogramConfirmDuration, durationInSeconds, nil)
}

func (mm *metricsManager) InitEventMetrics() {
	mm.eventsMetricsManager.NewCounterMetricWithLabels(mm.ctx, mtrCounterReceiptCheckTotal, mtrCounterReceiptCheckTotalDescription, []string{metricsLabelStatus}, false)
	mm.eventsMetricsManager.NewHistogramMetricWithLabels(mm.ctx, mtrHistogramReceiptCheckDuration, mtrHistogramReceiptCheckDurationDescription, []float64{} /*fallback to default buckets*/, []string{metricsLabelStatus}, false)

	mm.eventsMetricsManager.NewCounterMetricWithLabels(mm.ctx, mtrCounterConfirmationNotificationQueuedTotal, mtrCounterConfirmationNotificationQueuedTotalDescription, []string{metricsLabelType}, false)
	mm.eventsMetricsManager.NewHistogramMetricWithLabels(mm.ctx, mtrHistogramConfirmationNotificationQueueingDuration, mtrHistogramConfirmationNotificationQueueingDurationDescription, []float64{} /*fallback to default buckets*/, []string{metricsLabelType}, false)

	mm.eventsMetricsManager.NewCounterMetricWithLabels(mm.ctx, mtrCounterConfirmationNotificationProcessedTotal, mtrCounterConfirmationNotificationProcessedTotalDescription, []string{metricsLabelType}, false)
	mm.eventsMetricsManager.NewHistogramMetricWithLabels(mm.ctx, mtrHistogramConfirmationNotificationProcessDuration, mtrHistogramConfirmationNotificationProcessDurationDescription, []float64{} /*fallback to default buckets*/, []string{metricsLabelType}, false)

	mm.eventsMetricsManager.NewCounterMetric(mm.ctx, mtrCounterConfirmationBlockHashProcessedTotal, mtrCounterConfirmationBlockHashProcessedTotalDescription, false)
	mm.eventsMetricsManager.NewHistogramMetric(mm.ctx, mtrHistogramConfirmationBlockHashProcessDuration, mtrHistogramConfirmationBlockHashProcessDurationDescription, []float64{} /*fallback to default buckets*/, false)

	mm.eventsMetricsManager.NewCounterMetric(mm.ctx, mtrCounterConfirmedTotal, mtrCounterConfirmedTotalDescription, false)
	mm.eventsMetricsManager.NewHistogramMetric(mm.ctx, mtrHistogramConfirmDuration, mtrHistogramConfirmDurationDescription, []float64{} /*fallback to default buckets*/, false)

}
