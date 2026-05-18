export interface BaseAgentConfig {
  name: string;
  goal: string;
}

export class BaseAgent {
  constructor(public readonly config: BaseAgentConfig) {}
}
