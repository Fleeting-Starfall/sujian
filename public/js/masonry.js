/* 真·瀑布流 (JS 自实现, 零依赖)
 * 原理: 不依赖 CSS columns, 改用 JS 计算每列累计高度, 把卡片 append 到当前最短列,
 *       实现 Pinterest 风格「逐列从上到下、高度差互不干扰、新内容落最短列」。
 * 触发 reflow 的场景: 图片异步加载完成 (offsetHeight 变化) / 响应式断点切换 (resize)。
 */
'use strict';
(function (global) {
  'use strict';

  function throttle(fn, wait) {
    let last = 0, timer = null, lastArgs = null;
    return function (...args) {
      const now = Date.now();
      const remaining = wait - (now - last);
      lastArgs = args;
      if (remaining <= 0) {
        if (timer) { clearTimeout(timer); timer = null; }
        last = now;
        fn.apply(this, args);
      } else if (!timer) {
        timer = setTimeout(() => {
          last = Date.now();
          timer = null;
          fn.apply(this, lastArgs);
        }, remaining);
      }
    };
  }

  class Masonry {
    constructor(container, opts) {
      opts = opts || {};
      this.container = container;
      this.gap = (opts.gap != null) ? opts.gap : 16; // 与 CSS 列/卡片间距一致 (--space-4 = 16px)
      this.cards = [];       // 已渲染卡片(按追加顺序), reflow 时据此重排
      this.cols = 0;
      this.colEls = [];
      this.colHeights = [];  // 各列当前累计高度(含 gap), 用于选最短列
      this._reflowScheduled = false;
      this._onResize = throttle(() => this.reflow(), 200); // resize 节流 200ms
      global.addEventListener('resize', this._onResize);
    }

    // 根据容器宽度决定列数: ≥900 →4 / 480~900 →2 / <480 →1
    _colCount() {
      const w = this.container.clientWidth ||
                (document.documentElement && document.documentElement.clientWidth) ||
                global.innerWidth || 0;
      if (w >= 900) return 4;
      if (w >= 480) return 2;
      return 1;
    }

    // 创建/复用列容器; force=true 时无条件重建(清掉旧卡片)
    _setup(force) {
      const cols = this._colCount();
      if (!force && cols === this.cols && this.colEls.length === cols) return;
      this.cols = cols;
      this.colHeights = new Array(cols).fill(0);
      this.colEls = [];
      this.container.innerHTML = '';
      this.container.classList.add('masonry');
      for (let i = 0; i < cols; i++) {
        const c = document.createElement('div');
        c.className = 'masonry-col';
        this.container.appendChild(c);
        this.colEls.push(c);
      }
    }

    _shortestCol() {
      let idx = 0;
      for (let i = 1; i < this.colHeights.length; i++) {
        if (this.colHeights[i] < this.colHeights[idx]) idx = i;
      }
      return idx;
    }

    // 单卡片: 视频 hover 静音预览 + 图片加载完成触发重排
    _bindMedia(el) {
      const v = el.querySelector('video');
      if (v) {
        v.addEventListener('mouseenter', () => { try { v.play(); } catch (e) {} });
        v.addEventListener('mouseleave', () => { try { v.pause(); v.currentTime = 0; } catch (e) {} });
      }
      el.querySelectorAll('img').forEach(img => {
        if (img.complete && img.naturalWidth) {
          // 已缓存完成, 等下一帧重排即可
          this.reflow();
        } else {
          img.addEventListener('load', () => this.reflow(), { once: true });
          img.addEventListener('error', () => this.reflow(), { once: true });
        }
      });
    }

    _place(el) {
      const col = this._shortestCol();
      this.colEls[col].appendChild(el);
      const h = el.offsetHeight || 0;
      this.colHeights[col] += h + this.gap;
    }

    // 追加卡片元素(已构建好的 HTMLElement), 自动落入最短列
    append(els) {
      this._setup();
      const list = Array.isArray(els) ? els : [els];
      for (const el of list) {
        if (!(el instanceof global.HTMLElement)) continue;
        this.cards.push(el);
        this._bindMedia(el);
        this._place(el);
      }
    }

    // 全量重排(切换数据/频道/标签时调用): 清空并重建
    reset(els) {
      this.cards = [];
      this._setup(true);
      if (els) this.append(els);
    }

    // 重排: 所有卡片按原顺序重新分配到最短列 (图片加载完成 / 断点切换 -> 视觉平衡)
    reflow() {
      if (this._reflowScheduled) return; // 同帧内多次 onload 合并为一次
      this._reflowScheduled = true;
      requestAnimationFrame(() => {
        this._reflowScheduled = false;
        if (!this.cards.length) return;
        this._setup(); // 断点切换时重建列
        this.colHeights = new Array(this.cols).fill(0);
        for (const el of this.cards) {
          const col = this._shortestCol();
          this.colEls[col].appendChild(el);
          const h = el.offsetHeight || 0;
          this.colHeights[col] += h + this.gap;
        }
      });
    }

    destroy() {
      global.removeEventListener('resize', this._onResize);
    }
  }

  global.Masonry = Masonry;
  global.MasonryUtils = { throttle: throttle };
})(window);
