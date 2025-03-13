# Ferramenta Adaptativa para Configuração de Blockchain

## Visão Geral
Este projeto é uma ferramenta em Go projetada para otimizar dinamicamente os parâmetros da blockchain Hyperledger Fabric, como **Batch Timeout (BT)** e **Batch Size (BS)**. A ferramenta permite analisar a taxa de transações, atrasos e condições da rede para prever valores ideais e melhorar a eficiência.

## Funcionalidades
- Buscar dados de transações da blockchain Hyperledger Fabric.
- Calcular latência, taxa de transações e atraso da rede.
- Prever valores ideais para **Batch Timeout (BT)** e **Batch Size (BS)** usando algoritmos adaptativos.
- Modificar dinamicamente os parâmetros da blockchain.
- Executar testes de benchmark para análise de desempenho.
- Suporte a dois algoritmos de otimização: **FabMAN** e **aPBFT**.

## Requisitos
- Hyperledger Explorer instalado e configurado no ambiente.
- Go 1.18 ou superior.
- Docker e Docker Compose instalados.
- Rede Hyperledger Fabric em execução.
- `fabric-client` disponível no diretório especificado.
- Ferramentas CLI como `configtxlator` e `jq` instaladas.

## Instalação
1. Clone o repositório:
   ```sh
   git clone https://github.com/Ericksulino/adaptative.git
   cd adaptative
   ```
2. Verifique se o Go está instalado corretamente:
   ```sh
   go version
   ```
3. Compile o projeto (se necessário):
   ```sh
   go build -o adaptive adaptive.go
   ```

## Uso
O programa suporta vários modos de execução:

### 1. **Prever Parâmetros Ideais**
Prevê o melhor Batch Timeout e Batch Size com base no histórico da blockchain.
```sh
./adaptive -mode predict -algo fabman -block 2 -bt 2.0 -bs 10 -lambda 0.4 -tdelay 3.0
```

### 2. **Modificar Parâmetros da Blockchain**
Atualiza a configuração da Hyperledger Fabric com novos valores de Batch Timeout e Batch Size.
```sh
./adaptive -mode modify -bt 2.5 -bs 20
```

### 3. **Executar Benchmark**
Executa um teste de desempenho com uma carga específica de transações.
```sh
./adaptive -mode bench -tps 5 -numTx 100
```

### 4. **Benchmark Adaptativo (Ajuste Dinâmico de BS e BT)**
Executa múltiplos testes com cargas de transação crescentes.
```sh
./adaptive -mode benchAv -loads 5,10,15 -bt 2.0 -bs 10 -lambda 0.4 -tdelay 3.0 -algo fabman
```

## Opções de Linha de Comando
| Opção            | Descrição |
|------------------|-------------|
| `-mode`          | Modo de execução: `predict`, `modify`, `bench`, `benchAv` |
| `-algo`          | Algoritmo utilizado (`fabman` ou `apbft`) |
| `-block`         | Número do bloco a ser analisado |
| `-bt`            | Valor inicial de Batch Timeout (segundos) |
| `-bs`            | Valor inicial de Batch Size (quantidade de transações) |
| `-lambda`        | Fator de suavização EWMA para previsões |
| `-alpha`         | Fator alpha para cálculos do aPBFT |
| `-tdelay`        | Atraso máximo tolerado pelo sistema |
| `-tps`           | Transações por segundo para benchmarks |
| `-numTx`         | Número total de transações no benchmark |
| `-loads`         | Lista de cargas de transações separadas por vírgula |

## Exemplos de Uso
### Exemplo 1: Prever Parâmetros Ideais com FabMAN
```sh
./adaptive -mode predict -algo fabman -block 5 -bt 2.0 -bs 15 -lambda 0.4 -tdelay 3.0
```

### Exemplo 2: Modificar Configuração da Hyperledger Fabric
```sh
./adaptive -mode modify -bt 3.0 -bs 25
```

### Exemplo 3: Executar um Benchmark Simples
```sh
./adaptive -mode bench -tps 8 -numTx 200
```

### Exemplo 4: Executar Benchmark Adaptativo com Cargas Crescentes
```sh
./adaptive -mode benchAv -loads 10,20,30 -bt 2.0 -bs 10 -lambda 0.4 -tdelay 3.0 -algo apbft
```

## Notas
- Certifique-se de que a rede Hyperledger Fabric está ativa antes de executar a ferramenta.
- Permissões adequadas são necessárias para modificar a configuração da blockchain.
- Utilize valores apropriados de Batch Size e Batch Timeout conforme as condições da rede.

## Licença
Este projeto está licenciado sob a Licença MIT.

## Contato
Para dúvidas ou suporte, entre em contato com [seu email ou responsável pelo repositório].

