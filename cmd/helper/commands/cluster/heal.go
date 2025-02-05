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
*/
package cluster

import (
	"context"
	"crypto/sha1" // #nosec G505
	"fmt"
	"net"
	"os"
	"path"
	"strings"
	"time"

	"github.com/chideat/valkey-operator/cmd/helper/commands"
	"github.com/chideat/valkey-operator/pkg/valkey"
	"github.com/go-logr/logr"
	"github.com/urfave/cli/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type HealOptions struct {
	Namespace  string
	PodName    string
	Workspace  string
	TargetName string
	Prefix     string
	ShardID    string
	NodeFile   string
}

// Heal
func Heal(ctx context.Context, c *cli.Context, client *kubernetes.Clientset, logger logr.Logger) error {
	opts := &HealOptions{
		Namespace:  c.String("namespace"),
		PodName:    c.String("pod-name"),
		Workspace:  c.String("workspace"),
		TargetName: c.String("config-name"),
		Prefix:     c.String("prefix"),
		ShardID:    c.String("shard-id"),
		NodeFile:   path.Join(c.String("workspace"), c.String("config-name")),
	}

	var (
		err  error
		data []byte
	)

	logger.Info(fmt.Sprintf("check cluster status before pod %s startup", opts.PodName))
	if data, err = initClusterNodesConf(ctx, client, logger, opts); err != nil {
		logger.Error(err, "nit nodes.conf failed")
		return err
	}
	if data, err = portClusterNodesConf(ctx, data, logger, opts); err != nil {
		logger.Error(err, "port nodes.conf failed")
		return err
	}

	// persistent
	nodeFileBak := opts.NodeFile + ".bak"
	// back the nodes.conf to nodes.conf.bak
	_ = os.Rename(opts.NodeFile, nodeFileBak)

	tmpFile := path.Join(opts.Workspace, "tmp-"+opts.TargetName)
	if err := os.WriteFile(tmpFile, []byte(data), 0600); err != nil {
		logger.Error(err, "update nodes.conf failed")
		return err
	} else if err := os.Rename(tmpFile, opts.NodeFile); err != nil {
		logger.Error(err, "rename tmp-nodes.conf to nodes.conf failed")
		return err
	}
	return healCluster(c, ctx, client, data, logger)
}

func initClusterNodesConf(ctx context.Context, client *kubernetes.Clientset, logger logr.Logger, opts *HealOptions) ([]byte, error) {
	var (
		isUpdated = false
		data, err = os.ReadFile(opts.NodeFile)
	)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Error(err, "get nodes.conf failed")
			return nil, err
		}

		configName := strings.Join([]string{strings.TrimSuffix(opts.Prefix, "-"), opts.PodName}, "-")
		if data, err = getConfigMapData(ctx, client, opts.Namespace, configName, opts.TargetName, logger); err != nil {
			logger.Error(err, "sync nodes.conf from configmap to local failed")
			return nil, err
		}
		isUpdated = true
	}
	if len(data) == 0 {
		// if nodes.conf is empty, custom node-id and shard-id
		data = generateValkeyCluterNodeRecord(opts.Namespace, opts.PodName, opts.ShardID)
		isUpdated = true
	}
	if isUpdated && len(data) > 0 {
		return data, os.WriteFile(opts.NodeFile, data, 0644) // #nosec G306
	}
	return data, nil
}

func portClusterNodesConf(ctx context.Context, data []byte, logger logr.Logger, opts *HealOptions) ([]byte, error) {
	var (
		shardId   = opts.ShardID
		lines     []string
		epochLine string
	)

	// check node lines
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "vars") {
			if epochLine == "" {
				epochLine = line
			}
			continue
		}
		fields := strings.Fields(line)
		// NOTE: here ignore the wrong line, this may caused by valkey-server crash
		if !strings.Contains(line, "connect") || len(fields) < 8 {
			continue
		}
		lines = append(lines, line)
	}
	if epochLine != "" {
		// format: vars currentEpoch 105 lastVoteEpoch 105
		fields := strings.Fields(epochLine)
		if len(fields) == 5 {
			lines = append(lines, epochLine)
		} else if len(fields) == 4 && fields[3] == "lastVoteEpoch" {
			fields = append(fields, fields[2])
			epochLine = strings.Join(fields, " ")
			lines = append(lines, epochLine)
		} else if len(fields) == 3 && fields[1] == "currentEpoch" {
			fields = append(fields, "lastVoteEpoch", fields[2])
			epochLine = strings.Join(fields, " ")
			lines = append(lines, epochLine)
		}
		// ignore epoch line
	}

	data = []byte(strings.Join(lines, "\n"))
	if shardId == "" {
		return data, nil
	}

	// verify shard id
	lines = lines[0:0]
	nodes, err := valkey.ParseNodes(string(data))
	if err != nil {
		logger.Error(err, "parse nodes failed")
		return nil, err
	}
	selfNode := nodes.Self()
	masterNodes := map[string]*valkey.ClusterNode{}
	for _, node := range nodes.Masters() {
		if node.Id == selfNode.Id ||
			(selfNode.Role == valkey.MasterRole && node.MasterId == selfNode.Id) ||
			(selfNode.Role == valkey.SlaveRole && node.Id == selfNode.MasterId) {
			continue
		}
		masterNodes[node.Id] = node
	}

	for _, node := range nodes {
		line := node.Raw()
		if node.Id == selfNode.Id ||
			(selfNode.Role == valkey.MasterRole && node.MasterId == selfNode.Id) ||
			(selfNode.Role == valkey.SlaveRole && node.Id == selfNode.MasterId) {

			if oldShardId := node.AuxFields.ShardID; oldShardId != shardId {
				if oldShardId != "" {
					line = strings.ReplaceAll(line, fmt.Sprintf("shard-id=%s", oldShardId), fmt.Sprintf("shard-id=%s", shardId))
				} else {
					line = strings.ReplaceAll(line, node.AuxFields.Raw(), fmt.Sprintf("%s,,tls-port=0,shard-id=%s", node.AuxFields.Raw(), shardId))
				}
			}
			line = strings.ReplaceAll(line, ",nofailover", "")
			lines = append(lines, line)
		}
	}
	for _, master := range masterNodes {
		isAllReplicasInSameShard := true
		replicas := nodes.Replicas(master.Id)
		for _, replica := range replicas {
			if master.AuxFields.ShardID != replica.AuxFields.ShardID {
				isAllReplicasInSameShard = false
			}
		}
		if isAllReplicasInSameShard {
			lines = append(lines, master.Raw())
			for _, replica := range replicas {
				lines = append(lines, replica.Raw())
			}
		}
	}

	if len(lines) > 0 && epochLine != "" {
		lines = append(lines, epochLine)
	}
	return []byte(strings.Join(lines, "\n")), nil
}

func healCluster(c *cli.Context, ctx context.Context, client *kubernetes.Clientset, data []byte, logger logr.Logger) error {
	var (
		namespace = c.String("namespace")
		podName   = c.String("pod-name")
	)
	if namespace == "" {
		return fmt.Errorf("require namespace")
	}
	if podName == "" {
		return fmt.Errorf("require podname")
	}

	nodes, err := valkey.ParseNodes(string(data))
	if err != nil {
		logger.Error(err, "parse nodes failed")
		return err
	}
	nodesInfo, _ := nodes.Marshal()
	logger.Info("get nodes", "nodes", string(nodesInfo))

	if len(nodes) == 0 {
		return nil
	}

	var (
		self     = nodes.Self()
		replicas = nodes.Replicas(self.Id)
	)
	if !self.IsJoined() || len(replicas) == 0 {
		return nil
	}

	authInfo, err := commands.LoadAuthInfo(c, ctx)
	if err != nil {
		logger.Error(err, "load valkey operator user info failed")
		return err
	}

	masterExists := false
	// NOTE: when node is in importing state, if do force failover
	//       some slots will missing, or there will be multi master in cluster
	if slots := self.Slots(); !slots.IsImporting() {
		pods, _ := getPodsOfShard(ctx, c, client, logger)
		for _, pod := range pods {
			logger.Info(fmt.Sprintf("check pod %s", pod.GetName()))
			if pod.GetName() == podName {
				continue
			}
			if pod.GetDeletionTimestamp() != nil {
				continue
			}

			if err := func() error {
				addr := net.JoinHostPort(pod.Status.PodIP, "6379")
				announceIP := strings.ReplaceAll(pod.Labels["middleware.alauda.io/announce_ip"], "-", ":")
				announcePort := pod.Labels["middleware.alauda.io/announce_port"]
				if announceIP != "" && announcePort != "" {
					addr = net.JoinHostPort(announceIP, announcePort)
				}
				logger.Info("connect to valkey", "addr", addr)
				valkeyClient := valkey.NewValkeyClient(addr, *authInfo)
				defer valkeyClient.Close()

				nodes, err := valkeyClient.Nodes(ctx)
				if err != nil {
					logger.Error(err, "get nodes info failed")
					return err
				}
				currentNode := nodes.Self()
				if !currentNode.IsJoined() {
					logger.Info("unjoined node")
					return fmt.Errorf("unjoined node")
				}
				if currentNode.Role == valkey.MasterRole {
					// this shard has got one new master
					// clean and start
					logger.Info("master nodes exists")
					masterExists = true
					return nil
				}

				currentMaster := nodes.Get(currentNode.MasterId)
				if currentMaster != nil && (currentMaster.Id != self.Id &&
					!strings.Contains(currentMaster.RawFlag, "fail") &&
					!strings.Contains(currentMaster.RawFlag, "noaddr")) {
					masterExists = true
					return nil
				}
				if err := doValkeyFailover(ctx, valkeyClient, ForceFailoverAction, logger); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				continue
			}
			break
		}
	}

	if masterExists {
		// check current rdb and aof
		var (
			rdbFile       = "/data/dump.rdb"
			aofFile       = "/data/appendonly.aof"
			oldestModTime = time.Now().Add(time.Second * -3600)
		)
		if info, _ := os.Stat(rdbFile); info != nil {
			if info.ModTime().Before(oldestModTime) {
				// clean this file
				logger.Info("clean old dump.rdb")
				os.Remove(rdbFile)
			}
		}
		logger.Info("clean appendonly.aof")
		os.Remove(aofFile)
	}
	return nil
}

func generateValkeyCluterNodeRecord(namespace, podName, shardId string) []byte {
	tpl := `%s :0@0%s myself,master - 0 0 0 connected
vars currentEpoch 0 lastVoteEpoch 0`
	if shardId != "" {
		shardId = fmt.Sprintf(",,tls-port=0,shard-id=%s", shardId)
	}

	nodeId := fmt.Sprintf("%x", sha1.Sum([]byte(fmt.Sprintf("%s/%s", namespace, podName)))) // #nosec G401
	return []byte(fmt.Sprintf(tpl, nodeId, shardId))
}

func getPodsOfShard(ctx context.Context, c *cli.Context, client *kubernetes.Clientset, logger logr.Logger) (pods []corev1.Pod, err error) {
	var (
		namespace = c.String("namespace")
		podName   = c.String("pod-name")
	)
	if podName == "" {
		podName, _ = os.Hostname()
	}
	if podName == "" {
		return nil, nil
	}
	splitIndex := strings.LastIndex(podName, "-")
	stsName := podName[0:splitIndex]

	labels := map[string]string{
		"middleware.instance/type": "distributed-valkey-cluster",
		"statefulSet":              stsName,
	}

	if err = commands.RetryGet(func() error {
		if resp, err := client.CoreV1().Pods(namespace).List(ctx, v1.ListOptions{
			LabelSelector: v1.FormatLabelSelector(&v1.LabelSelector{MatchLabels: labels}),
			Limit:         5,
		}); err != nil {
			return err
		} else {
			pods = resp.Items
		}
		return nil
	}, 3); err != nil {
		logger.Error(err, "list statefulset pods failed")
		return nil, err
	}
	return
}

type FailoverAction string

const (
	NoFailoverAction    FailoverAction = ""
	ForceFailoverAction FailoverAction = "FORCE"
)

func doValkeyFailover(ctx context.Context, cli valkey.ValkeyClient, action FailoverAction, logger logr.Logger) (err error) {
	args := []interface{}{"FAILOVER"}
	if action != "" {
		args = append(args, action)
	}
	if _, err := cli.Do(ctx, "CLUSTER", args...); err != nil {
		logger.Error(err, "do failover failed", "action", action)
		return err
	}

	for i := 0; i < 3; i++ {
		logger.Info("check failover in 5s")
		time.Sleep(time.Second * 5)

		nodes, err := cli.Nodes(ctx)
		if err != nil {
			logger.Error(err, "fetch cluster nodes failed")
			return err
		}
		self := nodes.Self()
		if self == nil {
			return fmt.Errorf("get nodes info failed, as if the nodes.conf is broken")
		}
		if self.Role == valkey.MasterRole {
			logger.Info("failover succeed")
			return nil
		}
	}
	return fmt.Errorf("do manual failover failed")
}

func getConfigMapData(ctx context.Context, client *kubernetes.Clientset, namespace, name, target string, logger logr.Logger) ([]byte, error) {
	var cm *corev1.ConfigMap
	if err := commands.RetryGet(func() (err error) {
		cm, err = client.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
		return
	}, 20); errors.IsNotFound(err) {
		logger.Info("no synced nodes.conf found")
		return nil, nil
	} else if err != nil {
		logger.Error(err, "get configmap failed", "name", name)
		return nil, nil
	}
	return []byte(cm.Data[target]), nil
}
