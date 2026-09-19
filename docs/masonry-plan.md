# 真瀑布流改造方案

> 状态:**✅ 已开发实现 (方案 A: JS 自实现瀑布流)**。2026-08-20 落地: 新增 `public/js/masonry.js`(Masonry 类), `app.js` 的 `renderFeed` 改用 Masonry(`reset` 全量 / `appendFeed` 加载更多), 四个页面引入 `masonry.js`, `app.css` 移除 `columns` 改为 `.masonry` 弹性列容器。实现采用「列分发 + 最短列优先 + 图片 onload/resize 触发 reflow」(比原方案的绝对定位更鲁棒, 自动兼容 `loading="lazy"`)。

## 一、问题现状

当前首页瀑布流使用 **CSS 多列布局**(`columns: 4 230px`),存在以下固有限制:

| 现象               | 根因                                             |
| ---------------- | ---------------------------------------------- |
| 卡片高度差大时出现大空隙     | CSS `column-fill: balance`(默认)让各列底部对齐,短卡片被推到列底 |
| 内容很少时(2~3 篇)视觉空旷 | 浏览器按容器宽度硬性分 4 列,大量列空着                          |
| 新内容追加位置不可预测      | CSS 多列由浏览器决定填充顺序,新内容可能塞进任意列,影响用户"从上到下浏览"的预期    |
| 下架/删除内容后留"窟窿"    | CSS columns 不重新平衡已渲染内容                         |

## 二、目标

- ✅ 卡片按时间倒序**逐列从上到下填充**(Pinterest 风格),高度差互不干扰
- ✅ 新内容始终追加到**当前最短列**,保持视觉平衡
- ✅ 删除/下架内容后自动重排
- ✅ 响应式:≥900px 4 列 / 480~900 2 列 / <480 1 列
- ✅ 不引入第三方依赖(保持项目"零依赖"原则)

## 三、方案对比

### 方案 A:JS 自实现瀑布流(推荐)

**原理**:不依赖 CSS columns,改用 JS 计算每列高度,把卡片 append 到当前最短列。

**目录结构(新增)**:

```
public/js/
  app.js          (现有)
  masonry.js      (新增,~80 行)
```

**核心实现要点**:

```js
// 伪代码
class Masonry {
  constructor(container, opts) {
    this.container = container;
    this.cols = opts.cols; // 响应式断点决定
    this.gap = opts.gap || 12;
    this.colHeights = new Array(this.cols).fill(0);
  }
  append(item) {
    // 找到当前最短列,append 卡片到该列
    const col = this.shortestCol();
    this.colHeights[col] += item.offsetHeight + this.gap;
    // item 用 absolute 定位,top/left 按 col 算
  }
  reflow() { /* 全部重排,应对删除/图片加载完成 */ }
}
```

**关键细节**:

1. **图片异步高度**:笔记卡片内的 `<img>` 加载完成前 `offsetHeight` 不准。监听 `img.onload` → `reflow()` 重排。
2. **响应式列数**:`window.matchMedia` + `resize` 事件,断点切换时 `reflow()`。
3. **"加载更多"追加**:`append` 新卡片,自动落到最短列。
4. **下架/删除**:在 feed 层面已过滤,无需前端处理重排;但图片加载完成触发 reflow 即可让布局自动修复。
5. **分页**:复用现有 `feed.page` 参数;前端只需管理"是否还有更多"。

**改动范围**:

- 新增 `public/js/masonry.js`(~80 行,纯 DOM)
- `app.js` 的 `renderFeed` 改造:用 `Masonry` 替换直接 `innerHTML` / `insertAdjacentHTML`,不再调 `bindVideoHover`(在 Masonry 内统一处理)
- `index.html` / `search.html` / `tag.html` / `profile.html` 的 `renderFeed` 调用保持入口不变(传入容器 + 配置即可)
- 移除 CSS `.app-feed` 的 `columns/column-gap/column-fill`,改为单列容器(`.masonry-col`)
- CSS `.app-feed` → 拆成 `.masonry { position: relative; }` + `.masonry-col { position: absolute; ... }`

**性能**:

- 渲染 100 张卡片:首次 ~5ms(创建 DOM + 计算定位),图片加载后多次 reflow ~1ms/次
- resize 时 throttle 200ms,避免抖动

**风险**:

- 中等:重构 `renderFeed`,需全量回归 `index/search/tag/profile` 4 个页面
- 需要确认图片懒加载 (`loading="lazy"`) 与 `offsetHeight` 的兼容(lazy 图片未加载时 offsetHeight 偏小,reflow 时会正确撑开)

---

### 方案 B:CSS Grid `grid-template-rows: masonry`(实验性,暂不推荐)

**原理**:CSS 新提案 `masonry` 值,浏览器原生瀑布流。

**当前状态**(2026 年):

- 仅 Firefox Nightly 支持(behind flag)
- Chrome/Safari 不支持
- 短期不可用

**未来可作为长期方案**,届时移除 JS,性能更好。但目前需 polyfill,放弃。

---

### 方案 C:引入 Masonry.js(desandro)第三方库

**原理**:成熟的瀑布流库。

**优点**:功能完备、案例多。

**缺点**:

- **破坏项目"零依赖"原则**(README 强调 `纯 Go 标准库` + 无第三方包;前端加第三方库需权衡)
- 体积 ~15KB(gzip)
- API 与现有 `renderFeed` 风格不一致,需要适配层

**仅当方案 A 实现复杂时考虑**。

## 四、推荐方案 A 的实施步骤

1. **写 `masonry.js`**(核心类,~80 行)
2. **CSS 改造**:删 `.app-feed columns/column-gap/column-fill`,加 `.masonry` 容器 + `.masonry-col` 列容器
3. **`app.js` 改造 `renderFeed`**:引入 Masonry,管理实例,提供 `append(items)` / `reset()` / `reflow()`
4. **`index.html`**:首次 `loadFeed(true)` 用 `reset()`,`loadFeed(false)` 用 `append()`
5. **`search.html` / `tag.html`**:同上
6. **`profile.html`**:同 `renderFeed` 接入
7. **图片加载事件**:`img.onload` → `masonry.reflow()`
8. **响应式断点**:`window.resize` 监听,throttle 200ms,断点切换时 `reflow()`
9. **video hover 绑定**:整合进 Masonry 的 `append` 里
10. **测试**:
    - 不同卡片高度差下视觉对齐
    - 加载更多追加到最短列
    - 窄屏列数切换(1/2/4)
    - 删除笔记后重排

## 五、工作量评估

| 任务                   | 估计          |
| -------------------- | ----------- |
| `masonry.js` 核心实现    | 1~2 小时      |
| CSS 改造 + 4 个页面接入     | 2~3 小时      |
| 图片加载 + resize reflow | 1 小时        |
| 测试与回归                | 1~2 小时      |
| **总计**               | **0.5~1 天** |

## 六、风险与回退

- **风险**:JS 重构涉及多个页面的 `renderFeed` 调用,需回归
- **回退**:若效果不理想,CSS 恢复 `columns` + 删除 `masonry.js`(零依赖残留)
- **最小可行**:可以先只改 `index.html`,其他页面保留 CSS columns,验证后再铺开

