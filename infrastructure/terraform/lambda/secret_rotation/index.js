/**
 * AWS Secrets Manager rotation handler (placeholder).
 * Replace with per-key rotation logic before production.
 *
 * Steps: createSecret → setSecret → testSecret → finishSecret
 */
exports.handler = async (event) => {
  const arn = event.SecretId;
  const token = event.ClientRequestToken;
  const step = event.Step;

  console.log(JSON.stringify({ arn, token, step, status: "placeholder" }));

  if (step === "createSecret") {
    // In production: generate new ENGINE_ADMIN_SECRET, dual-write period
    return { status: "ok", step };
  }
  if (step === "setSecret") {
    return { status: "ok", step };
  }
  if (step === "testSecret") {
    return { status: "ok", step };
  }
  if (step === "finishSecret") {
    return { status: "ok", step };
  }

  throw new Error(`Unknown rotation step: ${step}`);
};
