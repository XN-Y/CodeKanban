import type { WatchSource, Ref, ComputedRef } from 'vue';
import { onBeforeRouteUpdate } from 'vue-router';
import { watch, onActivated, computed, watchEffect } from 'vue';
// import { makeOnError } from '@/utils/request';
import { useRequest, useAutoRequest } from 'alova/client';
import { useNotification } from 'naive-ui';
import type {
  AlovaMethodHandler,
  RequestHookConfig,
  UseHookExposure,
  AutoRequestHookConfig,
} from 'alova/client';
import type { AlovaGenerics, Method } from 'alova';

export const FORCE = Symbol('request force reload');

type UseReqConfig<AG extends AlovaGenerics, Args extends any[], X> = RequestHookConfig<AG, Args> & {
  skipShowError?: boolean;
  cacheFor?: number;
  onDataRefresh?: (data: Ref<AG['Responded']>) => void;
  computedFunc?: (data: Ref<AG['Responded']>) => X;
};

type UseReqExposure<AG extends AlovaGenerics, Args extends any[], X> = UseHookExposure<AG, Args> & {
  forceReload: UseHookExposure<AG, Args>['send'];
  dataComputed?: ComputedRef<X>;
  onDataRefresh: (onEvent: (data: Ref<AG['Responded']>) => void) => UseReqExposure<AG, Args, X>;
};

// 添加了默认配置的 useRequest 函数
// methodHandler: 一个带参的 Apis 请求，例如 (id: string) => Apis.approvalFlowTemplate.get({ pathParams: { id } }),
// config: 一般可省略，immediate默认值修改为false，也就是不立即发起请求，需要手动调用 send 方法
// config.skipShowError: 设为 true 时，不会显示错误通知，默认显示
// 对 send 的一点改造: 如果最后一个参数是 FORCE，会强制重新请求，而不是使用缓存数据
// 其他配置项参考 alova 官方文档
export function useReq<AG extends AlovaGenerics, Args extends any[], X = AG['Responded']>(
  methodHandler: Method<AG> | AlovaMethodHandler<AG, Args>,
  config?: UseReqConfig<AG, Args, X>
): UseReqExposure<AG, Args, X> {
  const notification = useNotification();
  let firstRequst = true;

  const r = useRequest(methodHandler, {
    immediate: false,

    async middleware(context, next) {
      if (firstRequst) {
        // 太抽象了
        context.method.config.cacheFor = config?.cacheFor || 5000; // 5s
        firstRequst = false;
      }
      return next();
    },

    force: ({ method, args }) => {
      return args[args.length - 1] === FORCE;
    },
    ...config,
  });

  if (!config?.skipShowError) {
    r.onError(error => {
      // TODO: 搞个错误处理类
      // makeOnError(notification, undefined, error)
    });
  }

  (r as any).forceReload = (...args: any[]) => {
    return r.send(...([...args, FORCE] as any));
  };

  if (config?.computedFunc) {
    const dataComputed = computed<X>(() => config.computedFunc!(r.data));
    (r as UseReqExposure<AG, Args, X>).dataComputed = dataComputed;
  }

  if (config?.onDataRefresh) {
    watchEffect(() => {
      config.onDataRefresh!(r.data);
    });
  }

  (r as UseReqExposure<AG, Args, X>).onDataRefresh = (
    func: (data: Ref<AG['Responded']>) => void
  ) => {
    watchEffect(() => {
      func(r.data);
    });
    return r as UseReqExposure<AG, Args, X>;
  };

  return r as UseReqExposure<AG, Args, X>;
}

// 初始化函数
// 一般用于初始化数据，例如从后端获取数据
// 如果挂上 watchSources，那么每次变更都会执行 init 函数
export function useInit(
  init: Function,
  watchSources: WatchSource<any>[] = [],
  options: Parameters<typeof watch>[2] = {}
) {
  // 1) 第一次 + sources 变化
  watch(watchSources, () => init(), { ...options });

  // 2) 路由参数变化（组件被复用）
  onBeforeRouteUpdate(() => init());

  // 3) KeepAlive 切回
  onActivated(() => init());

  init();
}

type UseAutoReqConfig<AG extends AlovaGenerics, Args extends any[]> = AutoRequestHookConfig<
  AG,
  Args
> & {
  skipShowError?: boolean;
};

// 自动请求 hook，支持轮询、页面可见性刷新、聚焦刷新等
// methodHandler: 一个带参的 Apis 请求
// config.pollingTime: 轮询时间（毫秒），默认 0 不轮询
// config.enableVisibility: 页面可见性变化时刷新，默认 true
// config.enableFocus: 页面聚焦时刷新，默认 true
// config.enableNetwork: 网络重连时刷新，默认 true
// config.throttle: 节流时间（毫秒），默认 1000
// config.skipShowError: 设为 true 时，不会显示错误通知
export function useAutoReq<AG extends AlovaGenerics, Args extends any[] = any[]>(
  methodHandler: Method<AG> | AlovaMethodHandler<AG, Args>,
  config?: UseAutoReqConfig<AG, Args>
): UseHookExposure<AG, Args> {
  const notification = useNotification();

  const r = useAutoRequest(methodHandler, {
    immediate: true,
    // 默认关闭缓存，因为自动请求需要获取最新数据
    cacheFor: 0,
    ...config,
  });

  if (!config?.skipShowError) {
    r.onError(error => {
      // TODO: 搞个错误处理类
      // makeOnError(notification, undefined, error)
    });
  }

  return r;
}
