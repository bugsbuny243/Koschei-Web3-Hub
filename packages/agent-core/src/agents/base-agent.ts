import type { AgentState } from "../types";

export abstract class BaseAgent {
  constructor(public readonly name: string) {}

  abstract run(input: AgentState): Promise<AgentState>;

  protected withLog(step: string, details: string) {
    console.log(`[${this.name}] ${step}: ${details}`);
  }
}
