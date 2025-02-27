package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	nethttp "net/http"
	"net/url"
	"strconv"
	"time"

	eth2client "github.com/attestantio/go-eth2-client"
	v1 "github.com/attestantio/go-eth2-client/api/v1"
	"github.com/attestantio/go-eth2-client/http"
	spec "github.com/attestantio/go-eth2-client/spec"
	"github.com/attestantio/go-eth2-client/spec/deneb"
	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/rs/zerolog"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/pk910/dora/rpc/sshtunnel"
	"github.com/pk910/dora/types"
	"github.com/pk910/dora/utils"
)

var logger = logrus.StandardLogger().WithField("module", "rpc")

type BeaconClient struct {
	name      string
	endpoint  string
	headers   map[string]string
	clientSvc eth2client.Service
	sshtunnel *sshtunnel.SSHTunnel
}

// NewBeaconClient is used to create a new beacon client
func NewBeaconClient(endpoint string, name string, headers map[string]string, sshcfg *types.EndpointSshConfig) (*BeaconClient, error) {
	client := &BeaconClient{
		name:     name,
		endpoint: endpoint,
		headers:  headers,
	}

	if sshcfg != nil {
		// create ssh tunnel to remote host
		sshPort := 0
		if sshcfg.Port != "" {
			sshPort, _ = strconv.Atoi(sshcfg.Port)
		}
		if sshPort == 0 {
			sshPort = 22
		}
		sshEndpoint := fmt.Sprintf("%v@%v:%v", sshcfg.User, sshcfg.Host, sshPort)
		var sshAuth ssh.AuthMethod
		if sshcfg.Keyfile != "" {
			var err error
			sshAuth, err = sshtunnel.PrivateKeyFile(sshcfg.Keyfile)
			if err != nil {
				return nil, fmt.Errorf("could not load ssh keyfile: %w", err)
			}
		} else {
			sshAuth = ssh.Password(sshcfg.Password)
		}

		// get tunnel target from endpoint url
		endpointUrl, _ := url.Parse(endpoint)
		tunTarget := endpointUrl.Host
		if endpointUrl.Port() != "" {
			tunTarget = fmt.Sprintf("%v:%v", tunTarget, endpointUrl.Port())
		} else {
			tunTargetPort := 80
			if endpointUrl.Scheme == "https:" {
				tunTargetPort = 443
			}
			tunTarget = fmt.Sprintf("%v:%v", tunTarget, tunTargetPort)
		}

		client.sshtunnel = sshtunnel.NewSSHTunnel(sshEndpoint, sshAuth, tunTarget)
		client.sshtunnel.Log = logger.WithField("sshtun", sshcfg.Host)
		err := client.sshtunnel.Start()
		if err != nil {
			return nil, fmt.Errorf("could not start ssh tunnel: %w", err)
		}

		// override endpoint to use local tunnel end
		endpointUrl.Host = fmt.Sprintf("localhost:%v", client.sshtunnel.Local.Port)

		client.endpoint = endpointUrl.String()
	}

	return client, nil
}

var errNotFound = errors.New("not found 404")

func (bc *BeaconClient) getJson(requrl string, returnValue interface{}) error {
	logurl := utils.GetRedactedUrl(requrl)
	t0 := time.Now()
	defer func() {
		logger.WithField("client", bc.name).Debugf("RPC GET call (json): %v [%v ms]", logurl, time.Since(t0).Milliseconds())
	}()

	req, err := nethttp.NewRequest("GET", requrl, nil)
	if err != nil {
		return err
	}
	for headerKey, headerVal := range bc.headers {
		req.Header.Set(headerKey, headerVal)
	}

	client := &nethttp.Client{Timeout: time.Second * 300}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != nethttp.StatusOK {
		if resp.StatusCode == nethttp.StatusNotFound {
			return errNotFound
		}
		data, _ := io.ReadAll(resp.Body)
		logger.WithField("client", bc.name).Debugf("RPC Error %v: %v", resp.StatusCode, data)
		return fmt.Errorf("url: %v, error-response: %s", logurl, data)
	}

	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&returnValue)
	if err != nil {
		return fmt.Errorf("error parsing json response: %v", err)
	}

	return nil
}

func (bc *BeaconClient) Initialize() error {
	if bc.clientSvc != nil {
		return nil
	}

	cliParams := []http.Parameter{
		http.WithAddress(bc.endpoint),
		http.WithTimeout(10 * time.Minute),
		// TODO (when upstream PR is merged)
		//http.WithConnectionCheck(false),
	}

	// set log level
	if utils.Config.Frontend.Debug {
		cliParams = append(cliParams, http.WithLogLevel(zerolog.InfoLevel))
	} else {
		cliParams = append(cliParams, http.WithLogLevel(zerolog.Disabled))
	}

	// set extra endpoint headers
	if bc.headers != nil && len(bc.headers) > 0 {
		cliParams = append(cliParams, http.WithExtraHeaders(bc.headers))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clientSvc, err := http.New(ctx, cliParams...)
	if err != nil {
		return err
	}

	bc.clientSvc = clientSvc
	return nil
}

func (bc *BeaconClient) GetGenesis() (*v1.Genesis, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider, isProvider := bc.clientSvc.(eth2client.GenesisProvider)
	if !isProvider {
		return nil, fmt.Errorf("get genesis not supported")
	}
	result, err := provider.Genesis(ctx)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (bc *BeaconClient) GetNodeSyncing() (*v1.SyncState, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider, isProvider := bc.clientSvc.(eth2client.NodeSyncingProvider)
	if !isProvider {
		return nil, fmt.Errorf("get node syncing not supported")
	}
	result, err := provider.NodeSyncing(ctx)
	if err != nil {
		return nil, err
	}
	return result, nil
}

type apiNodeVersion struct {
	Data struct {
		Version string `json:"version"`
	} `json:"data"`
}

func (bc *BeaconClient) GetNodeVersion() (string, error) {
	var nodeVersion apiNodeVersion
	err := bc.getJson(fmt.Sprintf("%s/eth/v1/node/version", bc.endpoint), &nodeVersion)
	if err != nil {
		return "", fmt.Errorf("error retrieving node version: %v", err)
	}
	return nodeVersion.Data.Version, nil
}

func (bc *BeaconClient) GetLatestBlockHead() (*v1.BeaconBlockHeader, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider, isProvider := bc.clientSvc.(eth2client.BeaconBlockHeadersProvider)
	if !isProvider {
		return nil, fmt.Errorf("get beacon block headers not supported")
	}
	result, err := provider.BeaconBlockHeader(ctx, "head")
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (bc *BeaconClient) GetFinalityCheckpoints() (*v1.Finality, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider, isProvider := bc.clientSvc.(eth2client.FinalityProvider)
	if !isProvider {
		return nil, fmt.Errorf("get finality not supported")
	}
	result, err := provider.Finality(ctx, "head")
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (bc *BeaconClient) GetBlockHeaderByBlockroot(blockroot []byte) (*v1.BeaconBlockHeader, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider, isProvider := bc.clientSvc.(eth2client.BeaconBlockHeadersProvider)
	if !isProvider {
		return nil, fmt.Errorf("get beacon block headers not supported")
	}
	result, err := provider.BeaconBlockHeader(ctx, fmt.Sprintf("0x%x", blockroot))
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (bc *BeaconClient) GetBlockHeaderBySlot(slot uint64) (*v1.BeaconBlockHeader, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider, isProvider := bc.clientSvc.(eth2client.BeaconBlockHeadersProvider)
	if !isProvider {
		return nil, fmt.Errorf("get beacon block headers not supported")
	}
	result, err := provider.BeaconBlockHeader(ctx, fmt.Sprintf("%d", slot))
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (bc *BeaconClient) GetBlockBodyByBlockroot(blockroot []byte) (*spec.VersionedSignedBeaconBlock, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider, isProvider := bc.clientSvc.(eth2client.SignedBeaconBlockProvider)
	if !isProvider {
		return nil, fmt.Errorf("get signed beacon block not supported")
	}
	result, err := provider.SignedBeaconBlock(ctx, fmt.Sprintf("0x%x", blockroot))
	if err != nil {
		return nil, err
	}
	return result, nil
}

type ProposerDuties struct {
	DependentRoot phase0.Root        `json:"dependent_root"`
	Data          []*v1.ProposerDuty `json:"data"`
}

func (bc *BeaconClient) GetProposerDuties(epoch uint64) (*ProposerDuties, error) {
	if utils.Config.Chain.WhiskForkEpoch != nil && epoch >= *utils.Config.Chain.WhiskForkEpoch {
		// whisk activated - cannot fetch proposer duties
		return nil, nil
	}

	var proposerDuties ProposerDuties
	err := bc.getJson(fmt.Sprintf("%s/eth/v1/validator/duties/proposer/%d", bc.endpoint, epoch), &proposerDuties)
	if err != nil {
		return nil, fmt.Errorf("error retrieving proposer duties: %v", err)
	}

	return &proposerDuties, nil
}

func (bc *BeaconClient) GetCommitteeDuties(stateRef string, epoch uint64) ([]*v1.BeaconCommittee, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider, isProvider := bc.clientSvc.(eth2client.BeaconCommitteesProvider)
	if !isProvider {
		return nil, fmt.Errorf("get beacon committees not supported")
	}
	result, err := provider.BeaconCommitteesAtEpoch(ctx, stateRef, phase0.Epoch(epoch))
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (bc *BeaconClient) GetSyncCommitteeDuties(stateRef string, epoch uint64) (*v1.SyncCommittee, error) {
	if epoch < utils.Config.Chain.Config.AltairForkEpoch {
		return nil, fmt.Errorf("cannot get sync committee duties for epoch before altair: %v", epoch)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider, isProvider := bc.clientSvc.(eth2client.SyncCommitteesProvider)
	if !isProvider {
		return nil, fmt.Errorf("get sync committees not supported")
	}
	result, err := provider.SyncCommitteeAtEpoch(ctx, stateRef, phase0.Epoch(epoch))
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (bc *BeaconClient) GetStateValidators(stateRef string) (map[phase0.ValidatorIndex]*v1.Validator, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider, isProvider := bc.clientSvc.(eth2client.ValidatorsProvider)
	if !isProvider {
		return nil, fmt.Errorf("get validators not supported")
	}
	result, err := provider.Validators(ctx, stateRef, nil)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (bc *BeaconClient) GetBlobSidecarsByBlockroot(blockroot []byte) ([]*deneb.BlobSidecar, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider, isProvider := bc.clientSvc.(eth2client.BeaconBlockBlobsProvider)
	if !isProvider {
		return nil, fmt.Errorf("get beacon block blobs not supported")
	}
	result, err := provider.BeaconBlockBlobs(ctx, fmt.Sprintf("0x%x", blockroot))
	if err != nil {
		return nil, err
	}
	return result, nil
}
