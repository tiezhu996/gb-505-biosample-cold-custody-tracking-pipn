import { LockOutlined, SafetyCertificateOutlined, UserOutlined } from '@ant-design/icons'
import { Alert, Button, Form, Input, Typography } from 'antd'
import { useState } from 'react'
import { Navigate, useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth'

export function LoginPage() {
  const { user, loading, login } = useAuth()
  const [error, setError] = useState('')
  const navigate = useNavigate()
  const location = useLocation()
  if (user) return <Navigate to="/specimens" replace />
  const submit = async (values: { username: string; password: string }) => {
    setError('')
    try {
      await login(values.username, values.password)
      const target = (location.state as { from?: string } | null)?.from || '/specimens'
      navigate(target, { replace: true })
    } catch { setError('登录失败，请核对账号和密码') }
  }
  return (
    <main className="login-page">
      <section className="login-intro"><SafetyCertificateOutlined /><Typography.Title>生物样本交接与冻存追踪</Typography.Title><Typography.Paragraph>让每一次接收、移交和协议复核都可追溯，让每一份科研样本处于正确温区和责任链中。</Typography.Paragraph></section>
      <section className="login-form-wrap">
        <div className="login-form">
          <Typography.Title level={2}>登录样本库工作台</Typography.Title>
          <Typography.Paragraph type="secondary">使用分配给你的接收、保管、复核或审计账号</Typography.Paragraph>
          {error && <Alert type="error" showIcon message={error} />}
          <Form layout="vertical" size="large" initialValues={{ username: 'admin', password: 'admin123' }} onFinish={(values) => void submit(values)}>
            <Form.Item name="username" label="账号" rules={[{ required: true }]}><Input prefix={<UserOutlined />} autoComplete="username" /></Form.Item>
            <Form.Item name="password" label="密码" rules={[{ required: true }]}><Input.Password prefix={<LockOutlined />} autoComplete="current-password" /></Form.Item>
            <Button htmlType="submit" type="primary" block loading={loading}>登录</Button>
          </Form>
        </div>
      </section>
    </main>
  )
}
