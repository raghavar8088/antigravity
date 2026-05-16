export class DeltaClientError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "DeltaClientError";
  }
}
