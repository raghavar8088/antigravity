package strategy

import (
	"log"
	
	"antigravity-engine/internal/marketdata"
	// "google.golang.org/grpc"
	
	// "antigravity-engine/internal/strategy/pb" // Uncomment after compiling protobuf with `protoc`!
)

// ExternalAI completely mimics our standard Mathematical Strategy interface, 
// except it routes the heavy computational load to a secondary Python server via hyper-fast gRPC.
type ExternalAI struct {
	serverAddress string
	name          string
	
	// client pb.AIServiceClient
}

func NewExternalAI(serverAddr string) *ExternalAI {
	// Active production connection logic:
	// conn, _ := grpc.Dial(serverAddr, grpc.WithInsecure())
	// client := pb.NewAIServiceClient(conn)
	
	return &ExternalAI{
		serverAddress: serverAddr,
		name:          "PyTorch_DRL_Alpha",
		// client: client,
	}
}

func (e *ExternalAI) Name() string { return e.name }

func (e *ExternalAI) OnTick(tick marketdata.Tick) []Signal {
	// =========================================================================
	// Placeholder logic until `protoc` is run and binaries exist locally.
	// =========================================================================
	log.Printf("[AI GATEWAY] Instantly bridging Tick(%.2f) -> Python(%s)\n", tick.Price, e.serverAddress)
	return []Signal{{Action: ActionHold}}
	
	// =========================================================================
	// True Architectural Implementation:
	// =========================================================================
	/*
	req := &pb.TickRequest{
		Symbol: tick.Symbol,
		Price: tick.Price,
		TimestampMs: tick.TimeMs,
	}
	
	resp, err := e.client.EvaluateTick(context.Background(), req)
	if err != nil {
		log.Printf("[AI SEVERITY] Failed to reach remote Python Neural server: %v", err)
		return []Signal{{Action: ActionHold}} // Default safely!
	}
	
	return []Signal{{
		Symbol: tick.Symbol,
		Action: Action(resp.Action),
		TargetSize: resp.TargetSize,
		Confidence: resp.Confidence,
	}}
	*/
}

func (e *ExternalAI) OnCandle(candle marketdata.Tick) []Signal {
	// In the real bridge, we would also expose an EvaluateCandle RPC endpoint.
	return e.OnTick(candle)
}
