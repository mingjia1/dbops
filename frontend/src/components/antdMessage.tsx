import { App } from 'antd'
import type { MessageInstance } from 'antd/es/message/interface'
import React from 'react'

let api: MessageInstance | null = null

// AntdMessageBridge 挂载在根 <AntApp> 内部, 把 App.useApp() 的 message 实例
// 交给下面的静态代理, 使全库 `import { message } from 'antd'` 的静态调用
// 不再触发 "[antd: message] Static function can not consume context" 警告,
// 同时让静态消息继承全局动态主题.
export const AntdMessageBridge: React.FC = () => {
  const { message: messageFromApp } = App.useApp()
  api = messageFromApp
  return null
}

type MessageMethod = MessageInstance['success']

function proxy(method: keyof MessageInstance): MessageMethod {
  return ((...args: Parameters<MessageMethod>) => {
    const fn = api?.[method] as MessageMethod | undefined
    return fn?.(...args)
  }) as MessageMethod
}

export const message: Pick<
  MessageInstance,
  'success' | 'error' | 'warning' | 'info' | 'loading' | 'open' | 'destroy'
> = {
  success: proxy('success'),
  error: proxy('error'),
  warning: proxy('warning'),
  info: proxy('info'),
  loading: proxy('loading'),
  open: ((config: Parameters<MessageInstance['open']>[0]) => api?.open(config)) as MessageInstance['open'],
  destroy: ((key?: React.Key) => api?.destroy(key)) as MessageInstance['destroy'],
}
