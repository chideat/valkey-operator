/*
Copyright 2024 chideat.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/package ops

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/ops/cluster"
	"github.com/chideat/valkey-operator/internal/ops/failover"
	"github.com/chideat/valkey-operator/internal/ops/sentinel"
	clustermodel "github.com/chideat/valkey-operator/internal/valkey/cluster"
	failovermodel "github.com/chideat/valkey-operator/internal/valkey/failover"
	sentinelmodel "github.com/chideat/valkey-operator/internal/valkey/sentinel"
	"github.com/chideat/valkey-operator/pkg/actor"
	"github.com/chideat/valkey-operator/pkg/kubernetes"
	"github.com/chideat/valkey-operator/pkg/kubernetes/clientset"
	"github.com/chideat/valkey-operator/pkg/types"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	MaxCallDepth                = 15
	DefaultRequeueDuration      = time.Second * 15
	DefaultAbortRequeueDuration = -1
	PauseRequeueDuration        = -1
	DefaultReconcileTimeout     = time.Minute * 5
)

type contextDepthKey struct{}
type contextLastCommandKey struct{}

var (
	DepthKey       = contextDepthKey{}
	LastCommandKey = contextLastCommandKey{}
)

type RuleEngine interface {
	Inspect(ctx context.Context, instance types.Instance) *actor.ActorResult
}

// OpEngine
type OpEngine struct {
	client        kubernetes.ClientSet
	eventRecorder record.EventRecorder

	clusterRuleEngine  RuleEngine
	failoverRuleEngine RuleEngine
	sentinelRuleEngine RuleEngine
	actorManager       *actor.ActorManager

	logger logr.Logger
}

// NewOpEngine
func NewOpEngine(client client.Client, eventRecorder record.EventRecorder, manager *actor.ActorManager, logger logr.Logger) (*OpEngine, error) {
	if client == nil {
		return nil, fmt.Errorf("require k8s clientset")
	}
	if eventRecorder == nil {
		return nil, fmt.Errorf("require k8s event recorder")
	}
	if manager == nil {
		return nil, fmt.Errorf("require actor manager")
	}

	e := OpEngine{
		client:        clientset.New(client, logger),
		eventRecorder: eventRecorder,
		actorManager:  manager,
		// logger:        logger.WithName("OpEngine"),
		logger: logger,
	}

	if engine, err := cluster.NewRuleEngine(e.client, eventRecorder, logger.WithName("ClusterOpEngine")); err != nil {
		return nil, err
	} else {
		e.clusterRuleEngine = engine
	}

	if engine, err := failover.NewRuleEngine(e.client, eventRecorder, logger.WithName("FailoverOpEngine")); err != nil {
		return nil, err
	} else {
		e.failoverRuleEngine = engine
	}

	if engine, err := sentinel.NewRuleEngine(e.client, eventRecorder, logger.WithName("SentinelOpEngine")); err != nil {
		return nil, err
	} else {
		e.sentinelRuleEngine = engine
	}
	return &e, nil
}

// Run
func (e *OpEngine) Run(ctx context.Context, val client.Object) (ctrl.Result, error) {
	logger := e.logger.WithName("OpEngine").WithValues("target", client.ObjectKeyFromObject(val))

	// each reconcile max run 5 minutes
	ctx, cancel := context.WithTimeout(ctx, DefaultReconcileTimeout)
	defer cancel()

	engine, inst, err := e.loadEngInst(ctx, val)
	if err != nil {
		logger.Error(err, "load instance failed")
		return ctrl.Result{}, err
	}
	return e.reconcile(ctx, engine, inst)
}

func (e *OpEngine) reconcile(ctx context.Context, engine RuleEngine, val types.Instance) (requeue ctrl.Result, err error) {
	logger := val.Logger()

	defer func() {
		if e := recover(); e != nil {
			logger.Error(fmt.Errorf("%s", e), "reconcile panic")
			err = fmt.Errorf("%s", e)
			debug.PrintStack()
		}
	}()

	var (
		failMsg         string
		status          = types.Any
		ret             = engine.Inspect(ctx, val)
		lastCommand     actor.Command
		requeueDuration time.Duration
	)

__end__:
	for depth := 0; ret != nil && depth <= MaxCallDepth; depth += 1 {
		if depth == MaxCallDepth {
			// use depth to limit the call black hole
			logger.Info(fmt.Sprintf("reconcile call depth exceeds %d, force requeue", MaxCallDepth), "target", val.GetName())

			requeueDuration = time.Second * 30
			break __end__
		}
		msg := fmt.Sprintf("run %s", ret.NextCommand().String())
		if lastCommand != nil {
			msg = fmt.Sprintf("run %s, last %s", ret.NextCommand().String(), lastCommand.String())
		}
		logger.Info(msg, "depth", depth)

		switch ret.NextCommand() {
		case actor.CommandAbort:
			requeueDuration, _ = ret.Result().(time.Duration)
			status, failMsg = types.Fail, ""
			if err := ret.Err(); err != nil {
				logger.Error(err, "reconcile aborted", "target", val.GetName())
				failMsg = err.Error()
			} else {
				logger.Info("reconcile aborted", "target", val.GetName())
			}
			if requeueDuration == 0 {
				requeueDuration = DefaultAbortRequeueDuration
			}
			break __end__
		case actor.CommandRequeue:
			requeueDuration, _ = ret.Result().(time.Duration)
			status, failMsg = types.Any, ""
			if err := ret.Err(); err != nil {
				logger.Error(err, "requeue with error", "target", val.GetName())
				failMsg = err.Error()
			} else {
				logger.V(3).Info("reconcile requeue", "target", val.GetName())
			}
			break __end__
		case actor.CommandPaused:
			requeueDuration, _ = ret.Result().(time.Duration)
			status, failMsg = types.Paused, ""
			if err := ret.Err(); err != nil {
				logger.Error(err, "pause failed with error", "target", val.GetName())
				failMsg = err.Error()
			} else {
				logger.V(3).Info("pause with no error", "target", val.GetName())
			}
			if requeueDuration == 0 {
				requeueDuration = PauseRequeueDuration
			}
			break __end__
		}

		ctx = context.WithValue(ctx, DepthKey, depth)
		if lastCommand != nil {
			ctx = context.WithValue(ctx, LastCommandKey, lastCommand.String())
		}

		ac := e.actorManager.Search(ret.NextCommand(), val)
		if ac == nil {
			err := fmt.Errorf("unknown command %s", ret.NextCommand())
			logger.Error(err, "actor for command not register")
			return ctrl.Result{}, err
		}
		lastCommand = ret.NextCommand()
		logger.V(3).Info(fmt.Sprintf("found actor %s with version %s", ret.NextCommand(), ac.Version()))

		if ret = ac.Do(ctx, val); ret == nil {
			// Re-inspect the instance to end the loop
			logger.V(3).Info("actor return nil, inspect one more time")
			ret = engine.Inspect(ctx, val)
		}
		if ret != nil && lastCommand != nil && ret.NextCommand().String() == lastCommand.String() {
			ret = actor.Requeue()
		}
	}

	if err := val.UpdateStatus(ctx, status, failMsg); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "update status before requeue failed")
	}
	if requeueDuration >= 0 {
		if requeueDuration == 0 {
			requeueDuration = DefaultRequeueDuration
		}
		return ctrl.Result{RequeueAfter: requeueDuration}, nil
	} else {
		return ctrl.Result{}, nil
	}
}

// loadEngInst
func (e *OpEngine) loadEngInst(ctx context.Context, obj client.Object) (engine RuleEngine, inst types.Instance, err error) {
	switch o := obj.(type) {
	case *v1alpha1.Cluster:
		inst, err = clustermodel.NewCluster(ctx, e.client, e.eventRecorder, o, e.logger)
		engine = e.clusterRuleEngine
	case *v1alpha1.Failover:
		inst, err = failovermodel.NewFailover(ctx, e.client, e.eventRecorder, o, e.logger)
		engine = e.failoverRuleEngine
	case *v1alpha1.Sentinel:
		inst, err = sentinelmodel.NewSentinel(ctx, e.client, e.eventRecorder, o, e.logger)
		engine = e.sentinelRuleEngine
	default:
		return nil, nil, fmt.Errorf("unknown type %T", obj)
	}
	return
}
