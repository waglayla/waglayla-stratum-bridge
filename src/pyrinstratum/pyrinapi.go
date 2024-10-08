package waglaylastratum

import (
	"context"
	"fmt"
	"time"

	"github.com/Waglayla/pyipad/app/appmessage"
	"github.com/Waglayla/pyipad/infrastructure/network/rpcclient"
	"github.com/Waglayla/waglayla-stratum-bride/src/gostratum"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type PyrinApi struct {
	address       string
	blockWaitTime time.Duration
	logger        *zap.SugaredLogger
	waglayla         *rpcclient.RPCClient
	connected     bool
}

func NewPyrinAPI(address string, blockWaitTime time.Duration, logger *zap.SugaredLogger) (*PyrinApi, error) {
	client, err := rpcclient.NewRPCClient(address)
	if err != nil {
		return nil, err
	}

	return &PyrinApi{
		address:       address,
		blockWaitTime: blockWaitTime,
		logger:        logger.With(zap.String("component", "waglaylaapi:"+address)),
		waglayla:         client,
		connected:     true,
	}, nil
}

func (py *PyrinApi) Start(ctx context.Context, blockCb func()) {
	py.waitForSync(true)
	go py.startBlockTemplateListener(ctx, blockCb)
	go py.startStatsThread(ctx)
}

func (py *PyrinApi) startStatsThread(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	for {
		select {
		case <-ctx.Done():
			py.logger.Warn("context cancelled, stopping stats thread")
			return
		case <-ticker.C:
			dagResponse, err := py.waglayla.GetBlockDAGInfo()
			if err != nil {
				py.logger.Warn("failed to get network hashrate from waglayla, prom stats will be out of date", zap.Error(err))
				continue
			}
			response, err := py.waglayla.EstimateNetworkHashesPerSecond(dagResponse.TipHashes[0], 1000)
			if err != nil {
				py.logger.Warn("failed to get network hashrate from waglayla, prom stats will be out of date", zap.Error(err))
				continue
			}
			RecordNetworkStats(response.NetworkHashesPerSecond, dagResponse.BlockCount, dagResponse.Difficulty)
		}
	}
}

func (py *PyrinApi) reconnect() error {
	if py.waglayla != nil {
		return py.waglayla.Reconnect()
	}

	client, err := rpcclient.NewRPCClient(py.address)
	if err != nil {
		return err
	}
	py.waglayla = client
	return nil
}

func (s *PyrinApi) waitForSync(verbose bool) error {
	if verbose {
		s.logger.Info("checking waglayla sync state")
	}
	for {
		clientInfo, err := s.waglayla.GetInfo()
		if err != nil {
			return errors.Wrapf(err, "error fetching server info from waglayla @ %s", s.address)
		}
		if clientInfo.IsSynced {
			break
		}
		s.logger.Warn("Pyrin is not synced, waiting for sync before starting bridge")
		time.Sleep(5 * time.Second)
	}
	if verbose {
		s.logger.Info("waglayla synced, starting server")
	}
	return nil
}

func (s *PyrinApi) startBlockTemplateListener(ctx context.Context, blockReadyCb func()) {
	blockReadyChan := make(chan bool)
	err := s.waglayla.RegisterForNewBlockTemplateNotifications(func(_ *appmessage.NewBlockTemplateNotificationMessage) {
		blockReadyChan <- true
	})
	if err != nil {
		s.logger.Error("fatal: failed to register for block notifications from waglayla")
	}

	ticker := time.NewTicker(s.blockWaitTime)
	for {
		if err := s.waitForSync(false); err != nil {
			s.logger.Error("error checking waglayla sync state, attempting reconnect: ", err)
			if err := s.reconnect(); err != nil {
				s.logger.Error("error reconnecting to waglayla, waiting before retry: ", err)
				time.Sleep(5 * time.Second)
			}
		}
		select {
		case <-ctx.Done():
			s.logger.Warn("context cancelled, stopping block update listener")
			return
		case <-blockReadyChan:
			blockReadyCb()
			ticker.Reset(s.blockWaitTime)
		case <-ticker.C: // timeout, manually check for new blocks
			blockReadyCb()
		}
	}
}

func (py *PyrinApi) GetBlockTemplate(
	client *gostratum.StratumContext) (*appmessage.GetBlockTemplateResponseMessage, error) {
	template, err := py.waglayla.GetBlockTemplate(client.WalletAddr,
		fmt.Sprintf(`'%s' via Waglayla/waglayla-stratum-bridge_%s`, client.RemoteApp, version))
	if err != nil {
		return nil, errors.Wrap(err, "failed fetching new block template from waglayla")
	}
	return template, nil
}
