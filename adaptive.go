package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
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
	Data struct {
		BlockNum  json.Number `json:"blocknum"` // Aceita números ou strings
		TxCount   int         `json:"txcount"`
		TxHashes  []string    `json:"txhash"`
		CreatedAt string      `json:"createdt"`
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

func createHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 15 * time.Second, // Tempo máximo de 15 segundos para a requisição
	}
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

func getBlockData(ip, channelGenesisHash, blockNumber, token string) (BlockResponse, error) {
	client := createHTTPClient()
	url := fmt.Sprintf("http://%s:8080/api/fetchDataByBlockNo/%s/%s", ip, channelGenesisHash, blockNumber)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return BlockResponse{}, fmt.Errorf("erro ao criar requisição HTTP: %v", err)
	}

	req.Header.Add("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return BlockResponse{}, fmt.Errorf("erro ao fazer requisição HTTP: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return BlockResponse{}, fmt.Errorf("status code: %d", resp.StatusCode)
	}

	var blockData BlockResponse
	if err := json.NewDecoder(resp.Body).Decode(&blockData); err != nil {
		return BlockResponse{}, fmt.Errorf("erro ao decodificar resposta JSON: %v", err)
	}

	// Exibir o BlockNum como string
	fmt.Printf("BlockNum recebido: %s\n", blockData.Data.BlockNum)

	return blockData, nil
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

func fetchBlockDataAndTransactions(serverIP string, blockNumber int, token string) (BlockResponse, BlockResponse, BlockResponse, []Transaction, error) {
	// Obter o channelGenesisHash
	channelGenesisHash, err := getChannelGenesisHash(serverIP, token)
	if err != nil {
		return BlockResponse{}, BlockResponse{}, BlockResponse{}, nil, fmt.Errorf("erro ao obter channelGenesisHash: %v", err)
	}

	// Validar se o blockNumber é válido
	if blockNumber < 0 {
		return BlockResponse{}, BlockResponse{}, BlockResponse{}, nil, fmt.Errorf("blockNumber inválido: %d", blockNumber)
	}

	// Obter os dados do bloco atual
	fmt.Printf("Buscando dados do bloco atual: %d\n", blockNumber)
	blockData, err := getBlockData(serverIP, channelGenesisHash, strconv.Itoa(blockNumber), token)
	if err != nil {
		return BlockResponse{}, BlockResponse{}, BlockResponse{}, nil, fmt.Errorf("erro ao obter informações do bloco atual: %v", err)
	}

	// Obter os dados do bloco anterior
	prevBlockNumber := blockNumber - 1
	if prevBlockNumber < 0 {
		return BlockResponse{}, BlockResponse{}, BlockResponse{}, nil, fmt.Errorf("número do bloco anterior inválido: %d", prevBlockNumber)
	}

	fmt.Printf("Buscando dados do bloco anterior: %d\n", prevBlockNumber)
	prevBlockData, err := getBlockData(serverIP, channelGenesisHash, strconv.Itoa(prevBlockNumber), token)
	if err != nil {
		return BlockResponse{}, BlockResponse{}, BlockResponse{}, nil, fmt.Errorf("erro ao obter informações do bloco anterior: %v", err)
	}

	// Obter os dados do bloco anterior ao anterior
	prevPrevBlockNumber := blockNumber - 2
	if prevPrevBlockNumber < 0 {
		return BlockResponse{}, BlockResponse{}, BlockResponse{}, nil, fmt.Errorf("número do bloco anterior ao anterior inválido: %d", prevPrevBlockNumber)
	}

	fmt.Printf("Buscando dados do bloco anterior ao anterior: %d\n", prevPrevBlockNumber)
	prevPrevBlockData, err := getBlockData(serverIP, channelGenesisHash, strconv.Itoa(prevPrevBlockNumber), token)
	if err != nil {
		return BlockResponse{}, BlockResponse{}, BlockResponse{}, nil, fmt.Errorf("erro ao obter informações do bloco anterior ao anterior: %v", err)
	}

	// Obter transações do bloco atual
	fmt.Println("Buscando transações do bloco atual...")
	var transactions []Transaction
	for _, txHash := range blockData.Data.TxHashes {
		txCreatedt, err := getTransactionCreatedt(serverIP, channelGenesisHash, txHash, token)
		if err != nil {
			fmt.Printf("Erro ao obter createdt da transação: %v\n", err)
			return BlockResponse{}, BlockResponse{}, BlockResponse{}, nil, fmt.Errorf("erro ao buscar transações: %v", err)
		}
		transactions = append(transactions, Transaction{
			TxHash:   txHash,
			Createdt: txCreatedt,
		})
	}

	// Caso haja menos de duas transações, continuar voltando blocos até achar um bloco válido
	if len(transactions) < 2 {
		fmt.Printf("Bloco %s possui apenas %d transações. Buscando bloco anterior...\n",
			blockData.Data.BlockNum, len(transactions))

		found := false
		current := blockNumber - 1

		for current >= 0 {
			fmt.Printf("Tentando bloco %d...\n", current)

			// Buscar dados do bloco atual da iteração
			tempBlock, err := getBlockData(serverIP, channelGenesisHash, strconv.Itoa(current), token)
			if err != nil {
				fmt.Printf("Erro ao obter bloco %d: %v\n", current, err)
				current--
				continue
			}

			// Buscar transações dele
			tempTxs := []Transaction{}
			for _, txHash := range tempBlock.Data.TxHashes {
				txCreatedt, err := getTransactionCreatedt(serverIP, channelGenesisHash, txHash, token)
				if err != nil {
					fmt.Printf("Erro ao obter createdt da tx %s: %v\n", txHash, err)
					continue
				}
				tempTxs = append(tempTxs, Transaction{
					TxHash:   txHash,
					Createdt: txCreatedt,
				})
			}

			// Se encontrou >= 2 transações, esse é o bloco que queremos
			if len(tempTxs) >= 2 {
				fmt.Printf("Bloco %d possui %d transações. Usando este bloco!\n", current, len(tempTxs))
				blockData = tempBlock
				transactions = tempTxs
				found = true
				break
			}

			fmt.Printf("Bloco %d possui apenas %d transações. Continuando...\n",
				current, len(tempTxs))

			current--
		}

		if !found {
			return BlockResponse{}, BlockResponse{}, BlockResponse{}, nil,
				fmt.Errorf("não foi encontrado nenhum bloco com transações suficientes")
		}
	}

	// Ordenar as transações por data de criação
	fmt.Println("Ordenando transações por timestamp...")
	sort.Slice(transactions, func(i, j int) bool {
		t1, err1 := time.Parse(time.RFC3339, transactions[i].Createdt)
		t2, err2 := time.Parse(time.RFC3339, transactions[j].Createdt)
		if err1 != nil || err2 != nil {
			fmt.Printf("Erro ao analisar timestamps: %v, %v\n", err1, err2)
			return false
		}
		return t1.Before(t2)
	})

	return blockData, prevBlockData, prevPrevBlockData, transactions, nil
}

// Função para calcular o tempo médio de transação (atraso)
func calculateAverageTransactionDelay(transactions []Transaction) (float64, error) {
	if len(transactions) == 0 {
		// Caso não haja transações
		return 0.0, fmt.Errorf("nenhuma transação disponível para calcular o atraso")
	}

	if len(transactions) == 1 {
		// Caso haja apenas uma transação
		txTime, err := time.Parse(time.RFC3339, transactions[0].Createdt)
		if err != nil {
			return 0.0, fmt.Errorf("erro ao converter timestamp da transação: %v", err)
		}

		// Calcula o atraso como o tempo desde a transação até agora
		delay := time.Since(txTime).Seconds()
		fmt.Printf("Atraso calculado com uma única transação: %.2f segundos\n", delay)
		return delay, nil
	}

	// Caso haja múltiplas transações, calcular o atraso médio
	firstTxTime, err := time.Parse(time.RFC3339, transactions[0].Createdt)
	if err != nil {
		return 0.0, fmt.Errorf("erro ao converter timestamp da primeira transação: %v", err)
	}
	lastTxTime, err := time.Parse(time.RFC3339, transactions[len(transactions)-1].Createdt)
	if err != nil {
		return 0.0, fmt.Errorf("erro ao converter timestamp da última transação: %v", err)
	}

	// Calcular o tempo total entre a primeira e a última transação
	timeDiff := lastTxTime.Sub(firstTxTime).Seconds()

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
	fmt.Printf("Calculando Treq...\n")
	fmt.Printf("txCout: %d\n", txCount)
	fmt.Printf("timeDiff: %2f\n", batchTimeout)
	if batchTimeout <= 0 {
		return 0.0
	}
	return float64(txCount) / batchTimeout
}

func calculateTimeBlocks(currentBlockTime string, prevBlockTime string) (float64, error) {
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

	return timeDiff, nil
}

func calculateNetworkDelay(currentBlockTime string, prevBlockTime string, batchTimeout float64) (float64, error) {
	// currentTime, err := time.Parse(time.RFC3339, currentBlockTime)
	// if err != nil {
	// 	return 0.0, err
	// }

	// prevTime, err := time.Parse(time.RFC3339, prevBlockTime)
	// if err != nil {
	// 	return 0.0, err
	// }

	// // Calcular o tempo entre blocos
	// timeDiff := currentTime.Sub(prevTime).Seconds()

	timeDiff, err := calculateTimeBlocks(currentBlockTime, prevBlockTime)
	if err != nil {
		fmt.Println("Erro ao calcular Ndelay do bloco atual:", err)
	}
	// Subtrair o Batch Timeout
	ndelay := timeDiff - batchTimeout
	return ndelay, nil
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
	fmt.Printf("Calculando proximo BS...\n")
	fmt.Printf("Alpha: %.2f\n", alpha)
	fmt.Printf("Treq: %.2f\n", treqPred)
	fmt.Printf("currentBS: %.2f\n", currentBatchSize)
	if treqPred >= alpha*currentBatchSize {
		return currentBatchSize * 2 // Dobrar BS
	} else if treqPred < alpha*(currentBatchSize/2) {
		return currentBatchSize / 2 // Reduzir pela metade
	}
	return currentBatchSize
}

// Função para executar cálculos e previsões do FabMAN
func processFabMAN(batchTimeout float64, batchSize int, tdelay, lambda float64, alpha float64, blockData, prevBlockData, prevPrevBlockData BlockResponse) (float64, float64) {
	timeDiff, err := calculateTimeBlocks(blockData.Data.CreatedAt, prevBlockData.Data.CreatedAt)
	// currentNdelay, err := calculateNetworkDelay(blockData.Data.CreatedAt, prevBlockData.Data.CreatedAt, batchTimeout)
	currentNdelay, err := calculateNetworkDelay(blockData.Data.CreatedAt, prevBlockData.Data.CreatedAt, timeDiff)
	if err != nil {
		fmt.Println("Erro ao calcular Ndelay do bloco atual:", err)
		return 0, 0 // Retornar valores padrão em caso de erro
	}

	prevTimeDiff, err := calculateTimeBlocks(prevBlockData.Data.CreatedAt, prevPrevBlockData.Data.CreatedAt)
	// prevNdelay, err := calculateNetworkDelay(prevBlockData.Data.CreatedAt, prevPrevBlockData.Data.CreatedAt, batchTimeout)
	prevNdelay, err := calculateNetworkDelay(prevBlockData.Data.CreatedAt, prevPrevBlockData.Data.CreatedAt, prevTimeDiff)
	if err != nil {
		fmt.Println("Erro ao calcular Ndelay do bloco anterior:", err)
		return 0, 0 // Retornar valores padrão em caso de erro
	}

	nextNdelay := calculateEWMA(currentNdelay, prevNdelay, lambda)
	nextBatchTimeout := calculateNextBatchTimeout(nextNdelay, tdelay)

	// prevTreq := calculateTransactionRequestRate(prevBlockData.Data.TxCount, batchTimeout)
	// treq := calculateTransactionRequestRate(blockData.Data.TxCount, batchTimeout)
	prevTreq := calculateTransactionRequestRate(prevBlockData.Data.TxCount, prevTimeDiff)
	treq := calculateTransactionRequestRate(blockData.Data.TxCount, timeDiff)
	nextTreq := calculateEWMA(treq, prevTreq, lambda)
	//nextBatchSize := calculateNextBatchSize(nextTreq, float64(batchSize)/batchTimeout, float64(blockData.Data.TxCount)) // "2.0" é um exemplo de alfa
	nextBatchSize := calculateNextBatchSize(nextTreq, 0.9, float64(batchSize)) // "2.0" é um exemplo de alfa

	// Prints
	fmt.Printf("----------fabMAN-------------\n")
	fmt.Printf("Ndelay Atual: %.2f segundos\n", currentNdelay)
	fmt.Printf("Ndelay Anterior: %.2f segundos\n", prevNdelay)
	fmt.Printf("Próximo Ndelay Previsto (EWMA): %.2f segundos\n", nextNdelay)
	fmt.Printf("Novo Batch Timeout previsto: %.2f segundos\n", nextBatchTimeout)
	fmt.Printf("Novo Batch Size previsto: %.2f\n", nextBatchSize)

	return nextBatchTimeout, nextBatchSize
}

// Função para calcular a Potência de Processamento (PW)
func calculateProcessingPower(tps float64, avgDelay float64) (float64, error) {
	if avgDelay <= 0 {
		return 0.0, fmt.Errorf("o atraso médio deve ser maior que 0 para calcular a potência de processamento")
	}
	return tps / avgDelay, nil
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

	// Predição do Batch Timeout (BT)
	var predictedBT float64
	if MTA >= currentMTE {
		predictedBT = 0.0 // Sistema está subutilizado
	} else {
		predictedBT = currentMTE
	}

	// Predição do Batch Size (BS)
	predictedBS := currentMTE / MTA

	// Garantir que os valores retornados sejam positivos e realistas
	if predictedBT < 0 || predictedBS < 0 {
		return 0.0, 0.0, fmt.Errorf("valores preditos inválidos: BT=%.2f, BS=%.2f", predictedBT, predictedBS)
	}

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

	return predictedBT, predictedBS
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
	// Impedir BatchTimeout inválido
	if newBatchTimeout <= 0 {
		fmt.Printf("BatchTimeout inválido (%.2f). Ajustando para mínimo de 0.10s\n", newBatchTimeout)
		newBatchTimeout = 0.10
	}
	// Ajustar newBatchSize se for menor que 1
	if newBatchSize < 1 {
		fmt.Println("newBatchSize is less than 1. Setting it to 1.")
		newBatchSize = 1
	}

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

// runCreateAssetBenchTime executa o comando para criar ativos no Fabric Client
func runCreateAssetBenchTime(tps int, time int) error {
	// Diretório onde o script está localizado (volta uma pasta antes de HLF_PET_go/)
	clientDir := "../HLF_PET_go/"

	// Montar o comando completo com o caminho relativo
	cmd := exec.Command(clientDir+"fabric-client", "createAssetBenchTime", fmt.Sprintf("%d", tps), fmt.Sprintf("%d", time))

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

	// Converter de volta para string
	return newBlockNumber
}

// Função auxiliar para encontrar o mínimo entre dois inteiros
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func CalculaTotalTransacoes(tps int, tempoSegundos int) int {
	// Transações Totais = TPS * Tempo em Segundos
	// Exemplo: 100 TPS * 10 segundos = 1000 Transações
	totalTransacoes := tps * tempoSegundos
	return totalTransacoes
}

func runBenchAv(serverIP string, token string, cargas []int, modeBench string, time int, batchTimeout, tdelay, lambda float64, batchSize, blockNumber, numTransactions int, algo string, alpha float64) {
	currentBT := batchTimeout
	currentBS := batchSize
	currentBlockNumber := blockNumber
	var transacoesParaCalculo int // Nova variável para ser usada no calculateBlockNumber

	modeBench = strings.TrimSpace(modeBench)

	for index, carga := range cargas {
		fmt.Printf("Processando carga %d: %d TPS Modo: %s\n", index, carga, modeBench)

		// Executar benchmark
		if modeBench == "numTrans" {
			//fmt.Printf("DEBUG: ENTROU NO MODO 'numTrans' Num: %d\n", numTransactions)
			if err := runCreateAssetBench(carga, numTransactions); err != nil {
				fmt.Println("Erro ao rodar o benchmark:", err)
				continue
			}
			transacoesParaCalculo = numTransactions
		} else if modeBench == "time" {
			//fmt.Printf("DEBUG: ENTROU NO MODO 'time' Time: %d\n", time)
			if err := runCreateAssetBenchTime(carga, time); err != nil {
				fmt.Println("Erro ao rodar o benchmark:", err)
				continue
			}
			transacoesParaCalculo = CalculaTotalTransacoes(carga, time)
		}

		// Calcular próximo número do bloco
		newBlockNumber := calculateBlockNumber(currentBlockNumber, currentBT, int(currentBS), transacoesParaCalculo, carga)
		fmt.Printf("Novo número do bloco calculado: %d\n", newBlockNumber)

		// Buscar dados do bloco atual
		fmt.Println("Buscando dados do bloco e transações...")
		blockData, prevBlockData, prevPrevBlockData, transactions, err := fetchBlockDataAndTransactions(serverIP, newBlockNumber-2, token)
		if err != nil {
			fmt.Printf("Erro ao buscar dados do bloco e transações: %v\n", err)
			continue
		}

		// Predizer os próximos Batch Timeout e Batch Size
		var newBT, newBS float64
		if algo == "fabman" {
			newBT, newBS = processFabMAN(currentBT, currentBS, tdelay, lambda, alpha, blockData, prevBlockData, prevPrevBlockData)
		} else {
			newBT, newBS = processAPBFT(transactions, currentBT, alpha, blockData)
		}

		// Modificar os parâmetros no sistema
		fmt.Printf("Alterando parâmetros: Batch Timeout = %.2f, Batch Size = %.2f\n", newBT, newBS)
		modifyParameters(newBT, int(newBS))

		// Atualizar valores para próxima iteração
		currentBT = newBT
		currentBS = int(newBS)
		currentBlockNumber = newBlockNumber
	}
}

func main() {
	mode := flag.String("mode", "predict", "Modo de operação: predict, modify, bench, benchAv")
	algo := flag.String("algo", "apbft", "Algoritmo: fabman ou apbft")
	blockNumber := flag.Int("block", 2, "Número do bloco para análise")
	batchTimeout := flag.Float64("bt", 2.0, "Batch Timeout atual (em segundos)")
	batchSize := flag.Int("bs", 10, "Batch Size atual (em número de mensagens)")
	lambda := flag.Float64("lambda", 0.4, "Fator de suavização para EWMA")
	alpha := flag.Float64("alpha", 0.4, "Fator alpha para previsão do aPBFT")
	tdelay := flag.Float64("tdelay", 3.0, "Latência máxima tolerada pelo sistema (Tdelay)")
	tps := flag.Int("tps", 5, "Transações por segundo (apenas para bench)")
	numTransactions := flag.Int("numTx", 100, "Número de transações totais (apenas para bench)")
	modeBench := flag.String("modeBench", "numTrans", "Modo de operação de Bench: predict, numTrans, time")
	time := flag.Int("time", 10, "Tempo de cada ciclo (apenas para bench)")
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
			processFabMAN(*batchTimeout, *batchSize, *tdelay, *lambda, *alpha, blockData, prevBlockData, prevPrevBlockData)
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
			// Converte o número do bloco para string
			newBlockNumber := calculateBlockNumber(*blockNumber, *batchTimeout, *batchSize, *numTransactions, *tps)
			fmt.Printf("Novo número do bloco: %d\n", newBlockNumber)
			fmt.Println("Benchmark executado com sucesso.")
		}
	case "benchAv":
		// Dividir as cargas fornecidas na flag
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

		// Obter o token JWT
		token, err := getJWTToken(serverIP)
		if err != nil {
			fmt.Printf("Erro ao obter token JWT no IP %s: %v\n", serverIP, err)
			return
		}

		// Chamar a função modularizada
		runBenchAv(
			serverIP,
			token,
			cargas,
			*modeBench,
			*time,
			*batchTimeout,
			*tdelay,
			*lambda,
			*batchSize,
			*blockNumber,
			*numTransactions,
			*algo,
			*alpha,
		)

	default:
		fmt.Println("Modo inválido. Escolha entre predict, modify, bench ou benchAv.")
	}
}
