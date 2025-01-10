package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Estrutura para representar uma transação
type Transaction struct {
	TxHash   string `json:"txhash"`
	Createdt string `json:"createdt"`
}

// Estrutura para resposta da API de transações do bloco
type BlockResponse struct {
	Status int `json:"status"`
	Data   struct {
		BlockNum  int      `json:"blocknum"`
		TxCount   int      `json:"txcount"`
		TxHashes  []string `json:"txhash"`
		CreatedAt string   `json:"createdt"` // Campo de data do bloco
	} `json:"data"`
}

// Estrutura para resposta da API de transação individual
type TransactionResponse struct {
	Status int `json:"status"`
	Row    struct {
		TxHash   string `json:"txhash"`
		Createdt string `json:"createdt"`
	} `json:"row"`
}

// Função para executar comandos no shell
func runCommand(command string) string {
	cmd := exec.Command("bash", "-c", command)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("Error executing command: %s\n", command)
		fmt.Printf("Error: %s\n", stderr.String())
		panic(err)
	}

	return out.String()
}

func getJWTToken(ip string) (string, error) {
	// Requisição de login
	loginPayload := `{"user":"exploreradmin","password":"exploreradminpw","network":"test-network"}`
	url := fmt.Sprintf("http://%s:8080/auth/login", ip) // Usar o IP fornecido
	resp, err := http.Post(url, "application/json", strings.NewReader(loginPayload))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Erro ao fazer login, status: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result["token"].(string), nil
}

// Função para obter o channel_genesis_hash
func getChannelGenesisHash(ip string, token string) (string, error) {
	req, err := http.NewRequest("GET", "http://"+ip+":8080/api/channels/info", nil)
	if err != nil {
		return "", err
	}

	// Adicionando o token JWT no cabeçalho
	req.Header.Add("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Erro ao obter informações do canal, status: %d", resp.StatusCode)
	}

	// Decodificando a resposta JSON
	var channelInfo struct {
		Status   int `json:"status"`
		Channels []struct {
			ChannelGenesisHash string `json:"channel_genesis_hash"`
		} `json:"channels"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&channelInfo); err != nil {
		return "", err
	}

	if len(channelInfo.Channels) > 0 {
		return channelInfo.Channels[0].ChannelGenesisHash, nil
	}

	return "", fmt.Errorf("Não foi possível encontrar o channel_genesis_hash")
}

// Função para obter o bloco com as transações
func getBlockData(ip string, channelGenesisHash string, blockNumber int, token string) (BlockResponse, error) {
	// URL da API de Bloco
	url := fmt.Sprintf("http://"+ip+":8080/api/fetchDataByBlockNo/%s/%d", channelGenesisHash, blockNumber)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return BlockResponse{}, err
	}

	// Adicionando o token JWT no cabeçalho
	req.Header.Add("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return BlockResponse{}, err
	}
	defer resp.Body.Close()

	// Verificar status da resposta
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Erro ao obter informações do bloco, status: %d\n", resp.StatusCode)
		fmt.Printf("Resposta: %s\n", string(body))
		return BlockResponse{}, fmt.Errorf("Erro ao obter informações do bloco, status: %d", resp.StatusCode)
	}

	// Parse da resposta JSON
	var result BlockResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return BlockResponse{}, err
	}

	return result, nil
}

// Função para obter a transação e seu createdt
func getTransactionCreatedt(ip string, channelGenesisHash, txHash, token string) (string, error) {
	// URL da API de Transação
	url := fmt.Sprintf("http://"+ip+":8080/api/transaction/%s/%s", channelGenesisHash, txHash)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	// Adicionando o token JWT no cabeçalho
	req.Header.Add("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Verificar status da resposta
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Erro ao obter informações da transação, status: %d", resp.StatusCode)
	}

	// Parse da resposta JSON
	var result TransactionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Row.Createdt, nil
}

// Função para calcular o tempo médio de transação (atraso)
func calculateAverageTransactionDelay(transactions []Transaction) (float64, error) {
	if len(transactions) < 2 {
		return 0.0, fmt.Errorf("não há transações suficientes para calcular o atraso")
	}

	// Converter o timestamp da primeira e última transação
	firstTxTime, err := time.Parse(time.RFC3339, transactions[0].Createdt)
	if err != nil {
		return 0.0, err
	}
	lastTxTime, err := time.Parse(time.RFC3339, transactions[len(transactions)-1].Createdt)
	if err != nil {
		return 0.0, err
	}

	// Calcular o tempo total entre a primeira e a última transação
	timeDiff := lastTxTime.Sub(firstTxTime).Seconds() // Tempo total em segundos

	// Calcular o tempo médio de transação (atraso)
	avgDelay := timeDiff / float64(len(transactions)-1)
	return avgDelay, nil
}

// Função para calcular transações por segundo (TPS)
func calculateTPS(transactions []Transaction) (float64, error) {
	if len(transactions) < 2 {
		return 0.0, fmt.Errorf("não há transações suficientes para calcular o TPS")
	}

	// Converter o timestamp da primeira e última transação
	firstTxTime, err := time.Parse(time.RFC3339, transactions[0].Createdt)
	if err != nil {
		return 0.0, err
	}
	lastTxTime, err := time.Parse(time.RFC3339, transactions[len(transactions)-1].Createdt)
	if err != nil {
		return 0.0, err
	}

	// Calcular o tempo total entre a primeira e a última transação
	timeDiff := lastTxTime.Sub(firstTxTime).Seconds() // Tempo total em segundos

	// Calcular a vazão (TPS)
	tps := float64(len(transactions)-1) / timeDiff
	return tps, nil
}

// Função para calcular a taxa de solicitação de transações (Treq)
func calculateTransactionRequestRate(txCount int, batchTimeout float64) float64 {
	if batchTimeout <= 0 {
		return 0.0
	}
	return float64(txCount) / batchTimeout
}

func calculateNetworkDelay(currentBlockTime string, prevBlockTime string, batchTimeout float64) (float64, error) {
	currentTime, err := time.Parse(time.RFC3339, currentBlockTime)
	if err != nil {
		return 0.0, err
	}

	prevTime, err := time.Parse(time.RFC3339, prevBlockTime)
	if err != nil {
		return 0.0, err
	}

	// Calcular o tempo entre blocos
	timeDiff := currentTime.Sub(prevTime).Seconds()

	// Subtrair o Batch Timeout
	ndelay := timeDiff - batchTimeout
	return ndelay, nil
}

// Função para calcular a Potência de Processamento (PW)
func calculateProcessingPower(tps float64, avgDelay float64) (float64, error) {
	if avgDelay <= 0 {
		return 0.0, fmt.Errorf("o atraso médio deve ser maior que 0 para calcular a potência de processamento")
	}
	return tps / avgDelay, nil
}

// EWMA para previsão de valores
func calculateEWMA(current, previous float64, lambda float64) float64 {
	return lambda*current + (1-lambda)*previous
}

// Função para calcular o próximo Batch Timeout (BT)
func calculateNextBatchTimeout(ndelayPred, tdelay float64) float64 {
	return tdelay - ndelayPred
}

// Função para ajustar o Batch Size (BS)
func calculateNextBatchSize(treqPred, alpha, currentBatchSize float64) float64 {
	if treqPred >= alpha {
		return currentBatchSize * 2 // Dobrar BS
	} else if treqPred < alpha/2 {
		return currentBatchSize / 2 // Reduzir pela metade
	}
	return currentBatchSize
}

func calculatePreviousMTE(currentMTE, TEk, alpha float64) (float64, error) {
	if alpha <= 0 || alpha >= 1 {
		return 0.0, fmt.Errorf("alpha deve estar no intervalo (0, 1)")
	}

	previousMTE := (currentMTE - alpha*TEk) / (1 - alpha)
	return previousMTE, nil
}

// Função para prever os parâmetros Batch Timeout (BT) e Batch Size (BS)
func predictBatchParameters(
	orderingTime, executionTime float64,
	successfulTransactions int,
	elapsedTime float64,
	previousMTE float64,
	alpha float64,
) (float64, float64, error) {
	// Validação das entradas
	if successfulTransactions <= 0 {
		return 0.0, 0.0, fmt.Errorf("número de transações bem-sucedidas deve ser maior que zero")
	}
	if orderingTime < 0 || executionTime < 0 || elapsedTime < 0 {
		return 0.0, 0.0, fmt.Errorf("tempos negativos são inválidos")
	}

	// Cálculo do tempo médio de execução por transação (TEk)
	TEk := (orderingTime + executionTime) / float64(successfulTransactions)

	// Atualização do MTE usando média móvel exponencial
	currentMTE := (1-alpha)*previousMTE + alpha*TEk

	// Cálculo do MTA (Mean Time Between Arrivals)
	MTA := elapsedTime / float64(successfulTransactions)
	if MTA <= 0 {
		return 0.0, 0.0, fmt.Errorf("MTA inválido ou zero, divisão impossível")
	}

	// Predição do Batch Timeout (BT) como MTE
	predictedBT := currentMTE

	// Predição do Batch Size (BS)
	predictedBS := currentMTE / MTA

	// Garantir que os valores retornados sejam positivos e realistas
	if predictedBT < 0 || predictedBS < 0 {
		return 0.0, 0.0, fmt.Errorf("valores preditos inválidos: BT=%.2f, BS=%.2f", predictedBT, predictedBS)
	}
	//fmt.Printf("orderingTime: %.4f, executionTime: %.4f\n", orderingTime, executionTime)
	//fmt.Printf("successfulTransactions: %d, elapsedTime: %.4f\n", successfulTransactions, elapsedTime)
	//fmt.Printf("TEk: %.4f, previousMTE: %.4f, alpha: %.4f\n", TEk, previousMTE, alpha)
	//fmt.Printf("MTA: %.4f\n", MTA)

	return predictedBT, predictedBS, nil
}

// Função para calcular o orderingTime e executionTime
func calculateTimes(transactions []Transaction) (float64, float64, error) {
	if len(transactions) < 2 {
		return 0.0, 0.0, fmt.Errorf("Número insuficiente de transações para calcular os tempos")
	}

	// Obter o timestamp da primeira e da última transação
	firstTimestamp, err1 := time.Parse(time.RFC3339, transactions[0].Createdt)
	lastTimestamp, err2 := time.Parse(time.RFC3339, transactions[len(transactions)-1].Createdt)

	if err1 != nil || err2 != nil {
		return 0.0, 0.0, fmt.Errorf("Erro ao analisar os timestamps: %v, %v", err1, err2)
	}

	// Calcular o orderingTime (tempo entre a primeira e a última transação)
	orderingTime := lastTimestamp.Sub(firstTimestamp).Seconds()

	// Calcular o executionTime (tempo médio por transação)
	var totalDiff float64
	for i := 1; i < len(transactions); i++ {
		prevTimestamp, err1 := time.Parse(time.RFC3339, transactions[i-1].Createdt)
		currTimestamp, err2 := time.Parse(time.RFC3339, transactions[i].Createdt)
		if err1 != nil || err2 != nil {
			return 0.0, 0.0, fmt.Errorf("Erro ao analisar timestamps durante o cálculo do executionTime: %v, %v", err1, err2)
		}
		totalDiff += currTimestamp.Sub(prevTimestamp).Seconds()
	}
	executionTime := totalDiff / float64(len(transactions)-1)

	return orderingTime, executionTime, nil
}

// Função para calcular o alpha com base em uma janela W
func calculateAlpha(W int) (float64, error) {
	if W <= 0 {
		return 0.0, fmt.Errorf("A janela W deve ser maior que zero")
	}
	return 1.0 / float64(W), nil
}

// Função para executar cálculos e previsões do FabMAN
func processFabMAN(batchTimeout, tdelay, lambda float64, blockData, prevBlockData, prevPrevBlockData BlockResponse) (float64, float64) {
	currentNdelay, err := calculateNetworkDelay(blockData.Data.CreatedAt, prevBlockData.Data.CreatedAt, batchTimeout)
	if err != nil {
		fmt.Println("Erro ao calcular Ndelay do bloco atual:", err)
		return 0, 0 // Retornar valores padrão em caso de erro
	}

	prevNdelay, err := calculateNetworkDelay(prevBlockData.Data.CreatedAt, prevPrevBlockData.Data.CreatedAt, batchTimeout)
	if err != nil {
		fmt.Println("Erro ao calcular Ndelay do bloco anterior:", err)
		return 0, 0 // Retornar valores padrão em caso de erro
	}

	nextNdelay := calculateEWMA(currentNdelay, prevNdelay, lambda)
	nextBatchTimeout := calculateNextBatchTimeout(nextNdelay, tdelay)

	prevTreq := calculateTransactionRequestRate(prevBlockData.Data.TxCount, batchTimeout)
	treq := calculateTransactionRequestRate(blockData.Data.TxCount, batchTimeout)
	nextTreq := calculateEWMA(treq, prevTreq, lambda)
	nextBatchSize := calculateNextBatchSize(nextTreq, 2.0, float64(blockData.Data.TxCount)) // "2.0" é um exemplo de alfa

	// Prints
	fmt.Printf("----------fabMAN-------------\n")
	fmt.Printf("Ndelay Atual: %.2f segundos\n", currentNdelay)
	fmt.Printf("Ndelay Anterior: %.2f segundos\n", prevNdelay)
	fmt.Printf("Próximo Ndelay Previsto (EWMA): %.2f segundos\n", nextNdelay)
	fmt.Printf("Novo Batch Timeout previsto: %.2f segundos\n", nextBatchTimeout)
	fmt.Printf("Novo Batch Size previsto: %.2f\n", nextBatchSize)

	return nextBatchTimeout, nextBatchSize
}

// Função para executar cálculos e previsões do aPBFT
func processAPBFT(transactions []Transaction, batchTimeout, alpha float64, blockData BlockResponse) (float64, float64) {
	orderingTime, executionTime, err := calculateTimes(transactions)
	if err != nil {
		fmt.Println("Erro ao calcular os tempos:", err)
		return 0, 0 // Retornar valores padrão em caso de erro
	}

	elapsedTime := orderingTime
	predictedBT, predictedBS, err := predictBatchParameters(orderingTime, executionTime, blockData.Data.TxCount, elapsedTime, batchTimeout, alpha)
	if err != nil {
		fmt.Println("Erro ao predizer parâmetros:", err)
		return 0, 0 // Retornar valores padrão em caso de erro
	}

	fmt.Printf("----------aPBFT-------------\n")
	fmt.Printf("Predicted Batch Timeout (BT): %.2f segundos\n", predictedBT)
	fmt.Printf("Predicted Batch Size (BS): %.2f\n", predictedBS)

	return predictedBS, predictedBT
}

func fetchBlockDataAndTransactions(serverIP string, blockNumber int, token string) (BlockResponse, BlockResponse, BlockResponse, []Transaction, error) {
	// Obter o channelGenesisHash
	channelGenesisHash, err := getChannelGenesisHash(serverIP, token)
	if err != nil {
		return BlockResponse{}, BlockResponse{}, BlockResponse{}, nil, fmt.Errorf("erro ao obter channelGenesisHash: %v", err)
	}

	// Obter os dados do bloco atual
	blockData, err := getBlockData(serverIP, channelGenesisHash, blockNumber, token)
	if err != nil {
		return BlockResponse{}, BlockResponse{}, BlockResponse{}, nil, fmt.Errorf("erro ao obter informações do bloco atual: %v", err)
	}

	// Obter os dados do bloco anterior
	prevBlockData, err := getBlockData(serverIP, channelGenesisHash, blockNumber-1, token)
	if err != nil {
		return BlockResponse{}, BlockResponse{}, BlockResponse{}, nil, fmt.Errorf("erro ao obter informações do bloco anterior: %v", err)
	}

	// Obter os dados do bloco anterior ao anterior
	prevPrevBlockData, err := getBlockData(serverIP, channelGenesisHash, blockNumber-2, token)
	if err != nil {
		return BlockResponse{}, BlockResponse{}, BlockResponse{}, nil, fmt.Errorf("erro ao obter informações do bloco anterior ao anterior: %v", err)
	}

	// Obter transações
	var transactions []Transaction
	for _, txHash := range blockData.Data.TxHashes {
		txCreatedt, err := getTransactionCreatedt(serverIP, channelGenesisHash, txHash, token)
		if err != nil {
			return BlockResponse{}, BlockResponse{}, BlockResponse{}, nil, fmt.Errorf("erro ao obter createdt da transação: %v", err)
		}
		transactions = append(transactions, Transaction{
			TxHash:   txHash,
			Createdt: txCreatedt,
		})
	}

	// Ordenar as transações por data de criação
	sort.Slice(transactions, func(i, j int) bool {
		t1, _ := time.Parse(time.RFC3339, transactions[i].Createdt)
		t2, _ := time.Parse(time.RFC3339, transactions[j].Createdt)
		return t1.Before(t2)
	})

	return blockData, prevBlockData, prevPrevBlockData, transactions, nil
}

func checkCurrentValues(newBatchTimeout float64, newBatchSize int) bool {
	fmt.Println("Step 1: Fetching current channel configuration...")
	runCommand("docker exec -e CHANNEL_NAME=mychannel cli sh -c 'peer channel fetch config config_block.pb -o orderer.example.com:7050 -c $CHANNEL_NAME --tls --cafile /opt/gopath/src/github.com/hyperledger/fabric/peer/crypto/ordererOrganizations/example.com/tlsca/tlsca.example.com-cert.pem'")
	runCommand("docker exec cli sh -c 'configtxlator proto_decode --input config_block.pb --type common.Block | jq .data.data[0].payload.data.config > config.json'")

	// Obter os valores atuais
	currentBatchSize := strings.TrimSpace(runCommand("docker exec cli sh -c 'jq .channel_group.groups.Orderer.values.BatchSize.value.max_message_count config.json'"))
	currentBatchTimeout := strings.TrimSpace(runCommand("docker exec cli sh -c 'jq .channel_group.groups.Orderer.values.BatchTimeout.value.timeout config.json'"))

	// Exibir valores atuais
	fmt.Printf("Current Max Message Count: %s\n", currentBatchSize)
	fmt.Printf("Current Batch Timeout: %s\n", currentBatchTimeout)

	// Comparar os valores
	formattedBatchTimeout := fmt.Sprintf("\"%.2fs\"", newBatchTimeout)
	batchSizeMatches := fmt.Sprintf("%d", newBatchSize) == currentBatchSize
	batchTimeoutMatches := formattedBatchTimeout == currentBatchTimeout

	// Se ambos forem iguais, exibir mensagem e interromper
	if batchSizeMatches && batchTimeoutMatches {
		fmt.Println("Both Batch Timeout and Max Message Count are already set to the desired values. No changes needed.")
		return false
	}

	// Continuar se pelo menos um valor precisar ser alterado
	if batchSizeMatches {
		fmt.Println("Max Message Count is already set to the desired value.")
	}
	if batchTimeoutMatches {
		fmt.Println("Batch Timeout is already set to the desired value.")
	}

	return true
}

func applyModifications(newBatchTimeout float64, newBatchSize int) {
	fmt.Println("Step 2: Applying modifications...")

	// Atualizar Max Message Count
	runCommand(fmt.Sprintf("docker exec cli sh -c 'jq \".channel_group.groups.Orderer.values.BatchSize.value.max_message_count = %d\" config.json > modified_config.json'", newBatchSize))

	// Atualizar Batch Timeout
	runCommand(fmt.Sprintf("docker exec cli sh -c 'jq \".channel_group.groups.Orderer.values.BatchTimeout.value.timeout = \\\"%.2fs\\\"\" modified_config.json > final_modified_config.json'", newBatchTimeout))

	fmt.Println("Converting JSON to ProtoBuf...")
	runCommand("docker exec cli sh -c 'configtxlator proto_encode --input config.json --type common.Config --output config.pb'")
	runCommand("docker exec cli sh -c 'configtxlator proto_encode --input final_modified_config.json --type common.Config --output modified_config.pb'")

	fmt.Println("Calculating Delta...")
	runCommand("docker exec -e CHANNEL_NAME=mychannel cli sh -c 'configtxlator compute_update --channel_id $CHANNEL_NAME --original config.pb --updated modified_config.pb --output final_update.pb'")

	fmt.Println("Adding update to envelope...")
	runCommand("docker exec cli sh -c 'configtxlator proto_decode --input final_update.pb --type common.ConfigUpdate | jq . > final_update.json'")
	runCommand("docker exec cli sh -c 'echo \"{\\\"payload\\\":{\\\"header\\\":{\\\"channel_header\\\":{\\\"channel_id\\\":\\\"mychannel\\\",\\\"type\\\":2}},\\\"data\\\":{\\\"config_update\\\":\"$(cat final_update.json)\"}}}\" | jq . > header_in_envelope.json'")
	runCommand("docker exec cli sh -c 'configtxlator proto_encode --input header_in_envelope.json --type common.Envelope --output final_update_in_envelope.pb'")
}

func updateChannel() {
	fmt.Println("Step 3: Updating channel...")

	// Assinar atualização
	runCommand("docker exec cli sh -c 'peer channel signconfigtx -f final_update_in_envelope.pb'")

	// Aplicar atualização
	runCommand("docker exec -e CORE_PEER_LOCALMSPID=OrdererMSP " +
		"-e CORE_PEER_TLS_ROOTCERT_FILE=/opt/gopath/src/github.com/hyperledger/fabric/peer/crypto/ordererOrganizations/example.com/orderers/orderer.example.com/tls/ca.crt " +
		"-e CORE_PEER_MSPCONFIGPATH=/opt/gopath/src/github.com/hyperledger/fabric/peer/crypto/ordererOrganizations/example.com/users/Admin@example.com/msp " +
		"-e CORE_PEER_ADDRESS=orderer.example.com:7050 " +
		"-e CHANNEL_NAME=mychannel " +
		"-e ORDERER_CA=/opt/gopath/src/github.com/hyperledger/fabric/peer/crypto/ordererOrganizations/example.com/tlsca/tlsca.example.com-cert.pem " +
		"cli sh -c 'peer channel update -f final_update_in_envelope.pb -c $CHANNEL_NAME -o orderer.example.com:7050 --tls --cafile $ORDERER_CA'")
}

func checkUpdatedValues() {
	fmt.Println("Step 4: Fetching updated channel configuration...")
	runCommand("docker exec -e CHANNEL_NAME=mychannel cli sh -c 'peer channel fetch config config_block.pb -o orderer.example.com:7050 -c $CHANNEL_NAME --tls --cafile /opt/gopath/src/github.com/hyperledger/fabric/peer/crypto/ordererOrganizations/example.com/tlsca/tlsca.example.com-cert.pem'")
	runCommand("docker exec cli sh -c 'configtxlator proto_decode --input config_block.pb --type common.Block | jq .data.data[0].payload.data.config > config.json'")

	updatedBatchSize := runCommand("docker exec cli sh -c 'jq .channel_group.groups.Orderer.values.BatchSize.value.max_message_count config.json'")
	updatedBatchTimeout := runCommand("docker exec cli sh -c 'jq .channel_group.groups.Orderer.values.BatchTimeout.value.timeout config.json'")

	fmt.Printf("Updated Max Message Count: %s\n", strings.TrimSpace(updatedBatchSize))
	fmt.Printf("Updated Batch Timeout: %s\n", strings.TrimSpace(updatedBatchTimeout))
}

func modifyParameters(newBatchTimeout float64, newBatchSize int) {
	// Verificar valores atuais
	if !checkCurrentValues(newBatchTimeout, newBatchSize) {
		return
	}

	// Aplicar modificações
	applyModifications(newBatchTimeout, newBatchSize)

	// Atualizar o canal
	updateChannel()

	// Verificar os valores atualizados
	checkUpdatedValues()

	// Finalizar com uma mensagem de sucesso
	fmt.Println("Batch size and batch timeout modification completed successfully.")
}

// runCreateAssetBench executa o comando para criar ativos no Fabric Client
func runCreateAssetBench(tps int, numTransactions int) error {
	// Diretório onde o script está localizado (volta uma pasta antes de HLF_PET_go/)
	clientDir := "../HLF_PET_go/"

	// Montar o comando completo com o caminho relativo
	cmd := exec.Command(clientDir+"fabric-client", "createAssetBench", fmt.Sprintf("%d", tps), fmt.Sprintf("%d", numTransactions))

	// Executar o comando
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Erro ao executar o comando: %v\n", err)
		fmt.Printf("Saída do comando: %s\n", string(output))
		return err
	}

	// Exibir a saída do comando
	fmt.Printf("Saída do comando:\n%s\n", string(output))
	return nil
}

func calculateBlockNumber(currentBlockNumber int, BT float64, BS int, numTransactions int, tps int) int {
	// Transações possíveis por bloco devido ao Batch Timeout
	transactionsPerBlockByBT := int(BT * float64(tps))

	// Transações por bloco é o menor valor entre BS e transactionsPerBlockByBT
	transactionsPerBlock := min(BS, transactionsPerBlockByBT)

	// Número de blocos necessários
	blocksNeeded := (numTransactions + transactionsPerBlock - 1) / transactionsPerBlock // Arredondar para cima

	// Novo número de bloco
	newBlockNumber := currentBlockNumber + blocksNeeded

	return newBlockNumber
}

// Função auxiliar para encontrar o mínimo entre dois inteiros
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func main() {
	mode := flag.String("mode", "predict", "Modo de operação: predict, modify, bench, benchAv")
	algo := flag.String("algo", "apbft", "Algoritmo: fabman ou apbft")
	blockNumber := flag.Int("block", 2, "Número do bloco para análise")
	batchTimeout := flag.Float64("bt", 2.0, "Batch Timeout atual (em segundos)")
	batchSize := flag.Int("bs", 10, "Batch Size atual (em número de mensagens)")
	lambda := flag.Float64("lambda", 0.3, "Fator de suavização para EWMA")
	alpha := flag.Float64("alpha", 0.1, "Fator alpha para previsão do aPBFT")
	tdelay := flag.Float64("tdelay", 3.0, "Latência máxima tolerada pelo sistema (Tdelay)")
	tps := flag.Int("tps", 100, "Transações por segundo (apenas para bench)")
	numTransactions := flag.Int("numTx", 100, "Número de transações totais (apenas para bench)")
	cargasFlag := flag.String("loads", "5,5,5", "Lista de cargas sepradas por vírgulas")
	flag.Parse()

	switch *mode {
	case "predict":
		// Previsão de parâmetros
		serverIP := `localhost`
		fmt.Printf("Utilizando IP: %s\n", serverIP)

		token, err := getJWTToken(serverIP)
		if err != nil {
			fmt.Printf("Erro ao obter token JWT no IP %s: %v\n", serverIP, err)
			return
		}

		blockData, prevBlockData, prevPrevBlockData, transactions, err := fetchBlockDataAndTransactions(serverIP, *blockNumber, token)
		if err != nil {
			fmt.Println("Erro ao buscar dados do bloco e transações:", err)
			return
		}

		switch *algo {
		case "fabman":
			processFabMAN(*batchTimeout, *tdelay, *lambda, blockData, prevBlockData, prevPrevBlockData)
		case "apbft":
			processAPBFT(transactions, *batchTimeout, *alpha, blockData)
		default:
			fmt.Println("Algoritmo inválido. Escolha entre fabman ou apbft.")
			return
		}

	case "modify":
		// Modificação de parâmetros
		fmt.Println("Modo modify selecionado")
		modifyParameters(*batchTimeout, *batchSize)

	case "bench":
		// Benchmark
		fmt.Printf("Iniciando benchmark com TPS=%d e numTransactions=%d\n", *tps, *numTransactions)
		err := runCreateAssetBench(*tps, *numTransactions)
		if err != nil {
			fmt.Println("Erro ao rodar o benchmark:", err)
		} else {
			newBlockNumber := calculateBlockNumber(*blockNumber, *batchTimeout, *batchSize, *numTransactions, *tps)
			fmt.Printf("Novo número do bloco: %d\n", newBlockNumber)
			fmt.Println("Benchmark executado com sucesso.")
		}
	case "benchAv":
		parts := strings.Split(*cargasFlag, ",")
		cargas := make([]int, len(parts))

		for i, part := range parts {
			num, err := strconv.Atoi(strings.TrimSpace(part))
			if err != nil {
				fmt.Printf("Erro ao converter carga '%s' para inteiro: %v\n", part, err)
				continue
			}
			cargas[i] = num
		}

		serverIP := `localhost`
		fmt.Printf("Utilizando IP: %s\n", serverIP)

		token, err := getJWTToken(serverIP)
		if err != nil {
			fmt.Printf("Erro ao obter token JWT no IP %s: %v\n", serverIP, err)
			return
		}

		for index, carga := range cargas {
			fmt.Printf("Processando carga %d: %d TPS\n", index, carga)

			err := runCreateAssetBench(carga, *numTransactions)
			if err != nil {
				fmt.Println("Erro ao rodar o benchmark:", err)
				continue
			}

			blockData, prevBlockData, prevPrevBlockData, transactions, err := fetchBlockDataAndTransactions(serverIP, *blockNumber, token)
			if err != nil {
				fmt.Println("Erro ao buscar dados do bloco e transações:", err)
				continue
			}

			bt, bs := 0.0, 0.0
			if *algo == "fabman" {
				bt, bs = processFabMAN(*batchTimeout, *tdelay, *lambda, blockData, prevBlockData, prevPrevBlockData)
			} else {
				bt, bs = processAPBFT(transactions, *batchTimeout, *alpha, blockData)
			}

			fmt.Printf("Novo Batch Timeout: %.2f, Novo Batch Size: %.2f\n", bt, bs)
			modifyParameters(bt, int(bs))
		}

	default:
		fmt.Println("Modo inválido. Escolha entre predict, modify ou bench.")
	}
}
