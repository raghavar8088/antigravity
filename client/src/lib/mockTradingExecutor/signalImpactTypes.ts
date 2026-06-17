export type SignalImpactStrategyRow = {
  name: string;
  strategyId: number;
  currentScore: number;
  signalThresholdPass: boolean;
  openThresholdPass: boolean;
  wouldQualify: boolean;
  gate: string;
};

export type SignalImpactReport = {
  accountKey: string;
  currentThreshold: number;
  testThreshold: number;
  currentMinSignalScore: number;
  testMinSignalScore: number;
  evaluatedStrategies: number;
  strategiesAboveSignalThreshold: number;
  strategiesAboveOpenThreshold: number;
  strategiesFullyQualified: number;
  strategies: SignalImpactStrategyRow[];
};
