package utils

// ============================================================
// 邮件 HTML 模板
// ============================================================

const verificationEmailTemplate = `
<html>
<body style="font-family: Arial, sans-serif; padding: 20px;">
  <div style="max-width: 480px; margin: 0 auto; background: #f9fafb; border-radius: 8px; padding: 24px;">
    <h2 style="color: #1a1a2e; margin-top: 0;">星海文学 · 邮箱验证</h2>
    <p>您的验证码是：</p>
    <div style="background: #fff; border: 1px solid #e5e7eb; border-radius: 6px; padding: 16px; text-align: center; margin: 16px 0;">
      <span style="font-size: 28px; font-weight: 700; letter-spacing: 6px; color: #2563eb;">%s</span>
    </div>
    <p style="color: #6b7280; font-size: 14px;">验证码 10 分钟内有效，请勿泄露给他人。</p>
    <p style="color: #9ca3af; font-size: 12px; margin-top: 24px;">如果这不是您本人的操作，请忽略此邮件。</p>
  </div>
</body>
</html>`

const passwordResetEmailTemplate = `
<html>
<body style="font-family: Arial, sans-serif; padding: 20px;">
  <div style="max-width: 480px; margin: 0 auto; background: #f9fafb; border-radius: 8px; padding: 24px;">
    <h2 style="color: #1a1a2e; margin-top: 0;">星海文学 · 密码重置</h2>
    <p>您的密码重置验证码是：</p>
    <div style="background: #fff; border: 1px solid #e5e7eb; border-radius: 6px; padding: 16px; text-align: center; margin: 16px 0;">
      <span style="font-size: 28px; font-weight: 700; letter-spacing: 6px; color: #dc2626;">%s</span>
    </div>
    <p style="color: #6b7280; font-size: 14px;">验证码 10 分钟内有效，请勿泄露给他人。</p>
    <p style="color: #9ca3af; font-size: 12px; margin-top: 24px;">如果这不是您本人的操作，请忽略此邮件。</p>
  </div>
</body>
</html>`
