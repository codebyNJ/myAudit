// Production S3 client adapter (MinIO locally via S3_ENDPOINT, real S3 in prod).
// Isolated so the storage facade and its tests need no AWS SDK. Not covered by
// the offline self-check.
import { S3Client, PutObjectCommand, GetObjectCommand } from '@aws-sdk/client-s3'

export function createS3Client({
  endpoint = process.env.S3_ENDPOINT, // e.g. http://localhost:9000 for MinIO
  region = process.env.S3_REGION || 'us-east-1',
  accessKeyId = process.env.S3_ACCESS_KEY,
  secretAccessKey = process.env.S3_SECRET_KEY,
} = {}) {
  const s3 = new S3Client({
    endpoint,
    region,
    forcePathStyle: true, // required for MinIO
    credentials: { accessKeyId, secretAccessKey },
  })
  return {
    async send(d) {
      if (d.op === 'PutObject') {
        return s3.send(new PutObjectCommand({ Bucket: d.Bucket, Key: d.Key, Body: d.Body, ContentType: d.ContentType }))
      }
      if (d.op === 'GetObject') {
        return s3.send(new GetObjectCommand({ Bucket: d.Bucket, Key: d.Key }))
      }
      throw new Error('unknown op: ' + d.op)
    },
  }
}
