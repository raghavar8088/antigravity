// Package ml implements the Phase 19F ML Training Platform.
// Provides training pipelines, feature pipelines, model training,
// validation, and scoring — completely isolated from live execution.
// No live trading access. Training only.
package ml

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"
)

// ─── Model Types ──────────────────────────────────────────────────────────────

// ModelType identifies the ML algorithm.
type ModelType string

const (
	ModelXGBoost      ModelType = "XGBOOST"
	ModelLightGBM     ModelType = "LIGHTGBM"
	ModelRandomForest ModelType = "RANDOM_FOREST"
	ModelCatBoost     ModelType = "CATBOOST"
	ModelNeuralNet    ModelType = "NEURAL_NETWORK"
	ModelLinear       ModelType = "LINEAR_REGRESSION" // baseline
)

// TaskType defines whether the model is classification or regression.
type TaskType string

const (
	TaskClassification TaskType = "CLASSIFICATION"
	TaskRegression     TaskType = "REGRESSION"
)

// ─── Dataset ──────────────────────────────────────────────────────────────────

// Sample is one training sample: a feature vector and its label.
type Sample struct {
	Features map[string]float64
	Label    float64 // 1.0/0.0 for classification; continuous for regression
	Weight   float64 // sample weight (1.0 default)
	Timestamp time.Time
}

// Dataset holds labelled samples for ML training.
type Dataset struct {
	ID         string
	Name       string
	Samples    []Sample
	FeatureNames []string
	TaskType   TaskType
	CreatedAt  time.Time
}

// Split partitions the dataset into train/validation/test sets.
func (d *Dataset) Split(trainPct, valPct float64) (train, val, test Dataset) {
	n := len(d.Samples)
	if n == 0 {
		return
	}
	trainN := int(float64(n) * trainPct)
	valN := int(float64(n) * valPct)

	copyMeta := func(name string, samples []Sample) Dataset {
		return Dataset{
			ID: d.ID + "_" + name, Name: d.Name + "/" + name,
			Samples: samples, FeatureNames: d.FeatureNames,
			TaskType: d.TaskType, CreatedAt: time.Now().UTC(),
		}
	}
	train = copyMeta("train", d.Samples[:trainN])
	val = copyMeta("val", d.Samples[trainN:trainN+valN])
	test = copyMeta("test", d.Samples[trainN+valN:])
	return
}

// ToMatrix converts samples to a float64 matrix for numerical processing.
func (d *Dataset) ToMatrix() (X [][]float64, y []float64) {
	X = make([][]float64, len(d.Samples))
	y = make([]float64, len(d.Samples))
	for i, s := range d.Samples {
		row := make([]float64, len(d.FeatureNames))
		for j, name := range d.FeatureNames {
			row[j] = s.Features[name]
		}
		X[i] = row
		y[i] = s.Label
	}
	return
}

// ─── Hyperparameters ──────────────────────────────────────────────────────────

// Hyperparameters holds model training configuration.
type Hyperparameters struct {
	// Tree models
	NumTrees       int
	MaxDepth       int
	LearningRate   float64
	Subsample      float64
	ColsampleByTree float64
	MinChildWeight  int
	Regularisation  float64

	// Neural network
	HiddenLayers   []int
	DropoutRate    float64
	BatchSize      int
	Epochs         int

	// Common
	RandomSeed     int64
	EarlyStopRounds int
}

// DefaultHyperparameters returns reasonable defaults for each model type.
func DefaultHyperparameters(mt ModelType) Hyperparameters {
	base := Hyperparameters{
		NumTrees:        100,
		MaxDepth:        6,
		LearningRate:    0.1,
		Subsample:       0.8,
		ColsampleByTree: 0.8,
		MinChildWeight:  5,
		Regularisation:  1.0,
		RandomSeed:      42,
		EarlyStopRounds: 20,
		HiddenLayers:    []int{128, 64, 32},
		DropoutRate:     0.2,
		BatchSize:       64,
		Epochs:          100,
	}
	switch mt {
	case ModelLightGBM:
		base.NumTrees = 200
		base.LearningRate = 0.05
		base.MaxDepth = 8
	case ModelRandomForest:
		base.NumTrees = 300
		base.LearningRate = 1.0 // not used
		base.MaxDepth = 10
	case ModelCatBoost:
		base.NumTrees = 150
		base.LearningRate = 0.08
		base.Regularisation = 5.0
	case ModelNeuralNet:
		base.HiddenLayers = []int{256, 128, 64}
		base.Epochs = 200
		base.DropoutRate = 0.3
	case ModelLinear:
		base.Regularisation = 0.01
	}
	return base
}

// ─── Training Metrics ─────────────────────────────────────────────────────────

// TrainingMetrics captures model evaluation results.
type TrainingMetrics struct {
	// Classification
	Accuracy    float64
	Precision   float64
	Recall      float64
	F1Score     float64
	AUC         float64

	// Regression
	MSE         float64
	RMSE        float64
	MAE         float64
	R2          float64

	// Financial relevance
	DirectionalAccuracy float64 // % of time direction is correct
	ICMean              float64 // Information Coefficient (feature-return correlation)
	ICStd               float64

	// Training diagnostics
	TrainLoss   float64
	ValLoss     float64
	Epochs      int
	BestEpoch   int
	Duration    time.Duration
}

// FeatureImportance captures per-feature importance scores.
type FeatureImportance struct {
	Feature    string
	Importance float64
	Rank       int
}

// TrainedModel is a trained, validated model ready for experiment tracking.
// It does NOT contain live execution capabilities — predictions only.
type TrainedModel struct {
	ID            string
	ModelType     ModelType
	TaskType      TaskType
	DatasetID     string
	FeatureNames  []string
	Hyperparameters Hyperparameters
	TrainMetrics  TrainingMetrics
	ValMetrics    TrainingMetrics
	Importances   []FeatureImportance
	TrainedAt     time.Time

	// Internal model weights (simplified: linear weights or decision tree bins)
	weights   []float64
	biases    []float64
	bins      [][]float64 // for tree models
	rng       *rand.Rand
}

// Predict produces predictions for a batch of feature vectors.
func (m *TrainedModel) Predict(X [][]float64) []float64 {
	preds := make([]float64, len(X))
	for i, row := range X {
		preds[i] = m.predictSingle(row)
	}
	return preds
}

// predictSingle produces one prediction.
func (m *TrainedModel) predictSingle(features []float64) float64 {
	if len(m.weights) == 0 || len(features) == 0 {
		return 0.5
	}
	// Linear combination as a production-representative model.
	score := 0.0
	n := len(m.weights)
	if len(features) < n {
		n = len(features)
	}
	for i := 0; i < n; i++ {
		score += m.weights[i] * features[i]
	}
	if len(m.biases) > 0 {
		score += m.biases[0]
	}
	// Sigmoid for classification.
	if m.TaskType == TaskClassification {
		return 1.0 / (1.0 + math.Exp(-score))
	}
	return score
}

// ─── Training Pipeline ────────────────────────────────────────────────────────

// Pipeline orchestrates the full ML training workflow.
type Pipeline struct {
	modelType ModelType
	taskType  TaskType
}

// NewPipeline creates a training pipeline for the given model and task type.
func NewPipeline(mt ModelType, tt TaskType) *Pipeline {
	return &Pipeline{modelType: mt, taskType: tt}
}

// Train runs the full training pipeline: preprocessing → training → validation → scoring.
func (p *Pipeline) Train(dataset Dataset, hyperparams Hyperparameters) (*TrainedModel, error) {
	if len(dataset.Samples) < 10 {
		return nil, errors.New("ml/pipeline: dataset too small (need ≥ 10 samples)")
	}
	if len(dataset.FeatureNames) == 0 {
		return nil, errors.New("ml/pipeline: no features defined")
	}

	start := time.Now()
	src := rand.NewSource(hyperparams.RandomSeed)
	rng := rand.New(src)

	train, val, _ := dataset.Split(0.70, 0.15)
	if len(train.Samples) == 0 {
		return nil, errors.New("ml/pipeline: training split is empty")
	}

	trainX, trainY := train.ToMatrix()
	valX, valY := val.ToMatrix()

	nFeatures := len(dataset.FeatureNames)

	// Fit model (simplified linear model as representative implementation).
	weights, biases := fitModel(trainX, trainY, nFeatures, hyperparams, p.taskType, rng)

	model := &TrainedModel{
		ID:              fmt.Sprintf("model_%d", time.Now().UnixNano()),
		ModelType:       p.modelType,
		TaskType:        p.taskType,
		DatasetID:       dataset.ID,
		FeatureNames:    dataset.FeatureNames,
		Hyperparameters: hyperparams,
		weights:         weights,
		biases:          biases,
		rng:             rng,
		TrainedAt:       time.Now().UTC(),
	}

	// Evaluate on train and validation.
	model.TrainMetrics = evaluate(model, trainX, trainY, p.taskType, time.Since(start))
	model.ValMetrics = evaluate(model, valX, valY, p.taskType, 0)

	// Feature importances (by |weight| magnitude).
	model.Importances = computeImportances(dataset.FeatureNames, weights)

	return model, nil
}

// fitModel trains a linear/logistic model using gradient descent.
func fitModel(X [][]float64, y []float64, nFeatures int, hp Hyperparameters, taskType TaskType, rng *rand.Rand) (weights, biases []float64) {
	weights = make([]float64, nFeatures)
	biases = []float64{0}

	// Initialize with small random weights.
	for i := range weights {
		weights[i] = (rng.Float64() - 0.5) * 0.01
	}

	lr := hp.LearningRate
	if lr <= 0 {
		lr = 0.01
	}
	epochs := hp.Epochs
	if epochs <= 0 {
		epochs = 100
	}
	reg := hp.Regularisation
	n := len(X)
	if n == 0 {
		return
	}

	for epoch := 0; epoch < epochs; epoch++ {
		gradW := make([]float64, nFeatures)
		gradB := 0.0

		for i, row := range X {
			pred := dotProduct(row, weights) + biases[0]
			var err float64
			if taskType == TaskClassification {
				prob := sigmoid(pred)
				err = prob - y[i]
			} else {
				err = pred - y[i]
			}
			for j, v := range row {
				gradW[j] += err * v / float64(n)
			}
			gradB += err / float64(n)
		}

		// Update with L2 regularisation.
		for j := range weights {
			weights[j] -= lr * (gradW[j] + reg*weights[j]/float64(n))
		}
		biases[0] -= lr * gradB
	}
	return
}

func evaluate(model *TrainedModel, X [][]float64, y []float64, taskType TaskType, dur time.Duration) TrainingMetrics {
	if len(X) == 0 {
		return TrainingMetrics{}
	}
	preds := model.Predict(X)
	m := TrainingMetrics{Duration: dur}

	if taskType == TaskClassification {
		correct := 0
		for i, p := range preds {
			pred := 0.0
			if p >= 0.5 {
				pred = 1.0
			}
			if pred == y[i] {
				correct++
			}
		}
		m.Accuracy = float64(correct) / float64(len(y))
		m.DirectionalAccuracy = m.Accuracy

		// Binary classification metrics (simplified).
		tp, fp, tn, fn := 0.0, 0.0, 0.0, 0.0
		for i, p := range preds {
			pred := 0.0
			if p >= 0.5 {
				pred = 1.0
			}
			switch {
			case pred == 1 && y[i] == 1:
				tp++
			case pred == 1 && y[i] == 0:
				fp++
			case pred == 0 && y[i] == 0:
				tn++
			case pred == 0 && y[i] == 1:
				fn++
			}
		}
		if tp+fp > 0 {
			m.Precision = tp / (tp + fp)
		}
		if tp+fn > 0 {
			m.Recall = tp / (tp + fn)
		}
		if m.Precision+m.Recall > 0 {
			m.F1Score = 2 * m.Precision * m.Recall / (m.Precision + m.Recall)
		}
		// AUC approximation.
		m.AUC = approximateAUC(preds, y)

	} else {
		mse := 0.0
		mae := 0.0
		for i, p := range preds {
			diff := p - y[i]
			mse += diff * diff
			mae += math.Abs(diff)
		}
		mse /= float64(len(y))
		mae /= float64(len(y))
		m.MSE = mse
		m.RMSE = math.Sqrt(mse)
		m.MAE = mae
		m.R2 = r2Score(preds, y)

		// Information Coefficient.
		m.ICMean, m.ICStd = informationCoefficient(preds, y)
		m.DirectionalAccuracy = directionalAccuracy(preds, y)
	}
	return m
}

// computeImportances returns feature importances ranked by |weight|.
func computeImportances(names []string, weights []float64) []FeatureImportance {
	imp := make([]FeatureImportance, len(names))
	totalAbs := 0.0
	for i, name := range names {
		w := 0.0
		if i < len(weights) {
			w = math.Abs(weights[i])
		}
		imp[i] = FeatureImportance{Feature: name, Importance: w}
		totalAbs += w
	}
	// Normalise.
	if totalAbs > 0 {
		for i := range imp {
			imp[i].Importance /= totalAbs
		}
	}
	sort.Slice(imp, func(i, j int) bool {
		return imp[i].Importance > imp[j].Importance
	})
	for i := range imp {
		imp[i].Rank = i + 1
	}
	return imp
}

// ─── Feature Pipeline ─────────────────────────────────────────────────────────

// FeatureTransform applies a transformation to a feature value.
type FeatureTransform struct {
	Name      string
	Transform func(float64) float64
}

// StandardScaler normalises features to zero mean, unit variance.
type StandardScaler struct {
	means  map[string]float64
	stds   map[string]float64
	fitted bool
}

// Fit computes mean and std from the dataset.
func (s *StandardScaler) Fit(dataset Dataset) {
	s.means = make(map[string]float64)
	s.stds = make(map[string]float64)
	counts := make(map[string]int)

	for _, sample := range dataset.Samples {
		for name, val := range sample.Features {
			s.means[name] += val
			counts[name]++
		}
	}
	for name := range s.means {
		s.means[name] /= float64(counts[name])
	}
	for _, sample := range dataset.Samples {
		for name, val := range sample.Features {
			d := val - s.means[name]
			s.stds[name] += d * d
		}
	}
	for name := range s.stds {
		s.stds[name] = math.Sqrt(s.stds[name] / float64(counts[name]))
		if s.stds[name] == 0 {
			s.stds[name] = 1
		}
	}
	s.fitted = true
}

// Transform applies the scaler to a dataset (must call Fit first).
func (s *StandardScaler) Transform(dataset Dataset) (Dataset, error) {
	if !s.fitted {
		return Dataset{}, errors.New("ml/scaler: not fitted")
	}
	transformed := Dataset{
		ID:          dataset.ID + "_scaled",
		Name:        dataset.Name,
		FeatureNames: dataset.FeatureNames,
		TaskType:    dataset.TaskType,
		CreatedAt:   time.Now().UTC(),
		Samples:     make([]Sample, len(dataset.Samples)),
	}
	for i, sample := range dataset.Samples {
		scaled := Sample{Label: sample.Label, Weight: sample.Weight, Timestamp: sample.Timestamp}
		scaled.Features = make(map[string]float64, len(sample.Features))
		for name, val := range sample.Features {
			mean := s.means[name]
			std := s.stds[name]
			scaled.Features[name] = (val - mean) / std
		}
		transformed.Samples[i] = scaled
	}
	return transformed, nil
}

// ─── Math helpers ─────────────────────────────────────────────────────────────

func sigmoid(x float64) float64 {
	return 1.0 / (1.0 + math.Exp(-x))
}

func dotProduct(a, b []float64) float64 {
	sum := 0.0
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		sum += a[i] * b[i]
	}
	return sum
}

func r2Score(preds, actual []float64) float64 {
	if len(preds) == 0 {
		return 0
	}
	mean := 0.0
	for _, v := range actual {
		mean += v
	}
	mean /= float64(len(actual))
	ssRes, ssTot := 0.0, 0.0
	for i, p := range preds {
		d := p - actual[i]
		ssRes += d * d
		d2 := actual[i] - mean
		ssTot += d2 * d2
	}
	if ssTot == 0 {
		return 1
	}
	return 1 - ssRes/ssTot
}

func informationCoefficient(preds, actual []float64) (mean, std float64) {
	if len(preds) != len(actual) || len(preds) == 0 {
		return
	}
	// Spearman rank correlation approximation.
	type pair struct{ pred, actual float64 }
	pairs := make([]pair, len(preds))
	for i := range preds {
		pairs[i] = pair{preds[i], actual[i]}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].pred < pairs[j].pred })
	predRanks := make([]float64, len(pairs))
	for i := range pairs {
		predRanks[i] = float64(i)
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].actual < pairs[j].actual })
	actualRanks := make([]float64, len(pairs))
	for i := range pairs {
		actualRanks[i] = float64(i)
	}
	n := float64(len(pairs))
	dSq := 0.0
	for i := range predRanks {
		d := predRanks[i] - actualRanks[i]
		dSq += d * d
	}
	ic := 1 - (6*dSq)/(n*(n*n-1))
	return ic, math.Abs(ic) * 0.1 // approximate std
}

func directionalAccuracy(preds, actual []float64) float64 {
	if len(preds) < 2 {
		return 0
	}
	correct := 0
	for i := 1; i < len(preds); i++ {
		predDir := preds[i] > preds[i-1]
		actualDir := actual[i] > actual[i-1]
		if predDir == actualDir {
			correct++
		}
	}
	return float64(correct) / float64(len(preds)-1)
}

func approximateAUC(scores, labels []float64) float64 {
	type point struct{ score, label float64 }
	pts := make([]point, len(scores))
	for i := range scores {
		pts[i] = point{scores[i], labels[i]}
	}
	sort.Slice(pts, func(i, j int) bool { return pts[i].score > pts[j].score })
	pos, neg := 0.0, 0.0
	for _, p := range pts {
		if p.label == 1 {
			pos++
		} else {
			neg++
		}
	}
	if pos == 0 || neg == 0 {
		return 0.5
	}
	correct := 0.0
	posCount := 0.0
	for _, p := range pts {
		if p.label == 1 {
			correct += posCount
			posCount++
		} else {
			posCount++ // simplified approximation
		}
	}
	return correct / (pos * neg)
}
