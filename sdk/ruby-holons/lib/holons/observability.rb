# Ruby reference implementation of the cross-SDK observability layer.
# Mirrors sdk/go-holons/pkg/observability. See OBSERVABILITY.md.

require 'fileutils'
require 'json'
require 'monitor'
require 'set'
require 'time'

module Holons
  module Observability
    FAMILIES = {
      logs: :logs, metrics: :metrics, events: :events, prom: :prom, otel: :otel
    }.freeze

    V1_TOKENS = %w[logs metrics events prom all].to_set.freeze

    class InvalidTokenError < StandardError
      attr_reader :token
      def initialize(token, reason)
        @token = token
        super("OP_OBS: #{reason}: #{token}")
      end
    end

    LEVELS = { trace: 1, debug: 2, info: 3, warn: 4, error: 5, fatal: 6 }.freeze
    LEVEL_LABELS = { 1 => 'TRACE', 2 => 'DEBUG', 3 => 'INFO',
                     4 => 'WARN', 5 => 'ERROR', 6 => 'FATAL' }.freeze

    EVENT_TYPES = {
      unspecified: 0, instance_spawned: 1, instance_ready: 2, instance_exited: 3,
      instance_crashed: 4, session_started: 5, session_ended: 6,
      handler_panic: 7, config_reloaded: 8
    }.freeze
    EVENT_TYPE_LABELS = {
      1 => 'INSTANCE_SPAWNED', 2 => 'INSTANCE_READY', 3 => 'INSTANCE_EXITED',
      4 => 'INSTANCE_CRASHED', 5 => 'SESSION_STARTED', 6 => 'SESSION_ENDED',
      7 => 'HANDLER_PANIC', 8 => 'CONFIG_RELOADED'
    }.freeze

    DEFAULT_BUCKETS = [
      50e-6, 100e-6, 250e-6, 500e-6,
      1e-3, 2.5e-3, 5e-3, 10e-3, 25e-3, 50e-3, 100e-3, 250e-3, 500e-3,
      1.0, 2.5, 5.0, 10.0, 30.0, 60.0
    ].freeze

    def self.parse_op_obs(raw)
      return Set.new if raw.nil? || raw.strip.empty?
      out = Set.new
      raw.split(',').each do |p|
        tok = p.strip
        next if tok.empty?
        next if tok == 'otel'
        next unless V1_TOKENS.include?(tok)
        if tok == 'all'
          out.merge(%i[logs metrics events prom])
        else
          out.add(tok.to_sym)
        end
      end
      out
    end

    def self.check_env(env = ENV)
      raw = (env['OP_OBS'] || '').strip
      return if raw.empty?
      raw.split(',').each do |p|
        tok = p.strip
        next if tok.empty?
        raise InvalidTokenError.new(tok, 'otel export is reserved for v2; not implemented in v1') if tok == 'otel'
        raise InvalidTokenError.new(tok, 'unknown OP_OBS token') unless V1_TOKENS.include?(tok)
      end
    end

    def self.append_direct_child(src, child_slug, child_uid)
      (src || []) + [{ slug: child_slug, instance_uid: child_uid }]
    end

    def self.enrich_for_multilog(wire, src_slug, src_uid)
      append_direct_child(wire, src_slug, src_uid)
    end

    # --- LogRing ---

    class LogRing
      include MonitorMixin
      attr_reader :capacity

      def initialize(capacity = 1024)
        super()
        @capacity = [1, capacity].max
        @buf = []
        @subs = []
      end

      def push(entry)
        subs_copy = nil
        synchronize do
          @buf << entry
          @buf.shift if @buf.size > @capacity
          subs_copy = @subs.dup
        end
        subs_copy.each do |fn|
          begin; fn.call(entry); rescue => _; end
        end
      end

      def drain; synchronize { @buf.dup }; end
      def drain_since(cutoff); synchronize { @buf.select { |e| e[:timestamp] >= cutoff } }; end
      def size; synchronize { @buf.size }; end

      def subscribe(&fn)
        synchronize { @subs << fn }
        -> { synchronize { @subs.delete(fn) } }
      end
    end

    # --- EventBus ---

    class EventBus
      include MonitorMixin

      def initialize(capacity = 256)
        super()
        @capacity = [1, capacity].max
        @buf = []
        @subs = []
        @closed = false
      end

      def emit(event)
        subs_copy = nil
        synchronize do
          return if @closed
          @buf << event
          @buf.shift if @buf.size > @capacity
          subs_copy = @subs.dup
        end
        subs_copy.each { |fn| begin; fn.call(event); rescue; end }
      end

      def drain; synchronize { @buf.dup }; end
      def drain_since(cutoff); synchronize { @buf.select { |e| e[:timestamp] >= cutoff } }; end

      def subscribe(&fn)
        synchronize { @subs << fn }
        -> { synchronize { @subs.delete(fn) } }
      end

      def close; synchronize { @closed = true; @subs.clear }; end
    end

    # --- Metrics ---

    class Counter
      include MonitorMixin
      attr_reader :name, :help, :labels
      def initialize(name, help, labels)
        super()
        @name = name; @help = help; @labels = labels
        @value = 0
      end
      def inc(n = 1); return if n < 0; synchronize { @value += n }; end
      def add(n); inc(n); end
      def value; synchronize { @value }; end
    end

    class Gauge
      include MonitorMixin
      attr_reader :name, :help, :labels
      def initialize(name, help, labels)
        super()
        @name = name; @help = help; @labels = labels
        @value = 0.0
      end
      def set(v); synchronize { @value = v.to_f }; end
      def add(d); synchronize { @value += d.to_f }; end
      def value; synchronize { @value }; end
    end

    class HistogramSnapshot
      attr_reader :bounds, :counts, :total, :sum
      def initialize(bounds, counts, total, sum)
        @bounds = bounds; @counts = counts; @total = total; @sum = sum
      end

      def quantile(q)
        return Float::NAN if @total.zero?
        target = @total * q
        @counts.each_with_index { |c, i| return @bounds[i] if c >= target }
        Float::INFINITY
      end
    end

    class Histogram
      include MonitorMixin
      attr_reader :name, :help, :labels

      def initialize(name, help, labels, bounds)
        super()
        @name = name; @help = help; @labels = labels
        b = (bounds.nil? || bounds.empty?) ? DEFAULT_BUCKETS.dup : bounds.dup
        @bounds = b.sort
        @counts = Array.new(@bounds.size, 0)
        @total = 0
        @sum = 0.0
      end

      def observe(v)
        synchronize do
          @total += 1
          @sum += v
          @bounds.each_with_index { |b, i| @counts[i] += 1 if v <= b }
        end
      end

      def observe_duration(seconds); observe(seconds); end

      def snapshot
        synchronize { HistogramSnapshot.new(@bounds.dup, @counts.dup, @total, @sum) }
      end
    end

    def self.metric_key(name, labels)
      return name if labels.nil? || labels.empty?
      "#{name}|" + labels.sort.map { |k, v| "#{k}=#{v}" }.join(',')
    end

    class Registry
      include MonitorMixin
      def initialize
        super()
        @counters = {}
        @gauges = {}
        @histograms = {}
      end
      def counter(name, help = '', labels = {})
        k = Observability.metric_key(name, labels)
        synchronize { @counters[k] ||= Counter.new(name, help, labels.dup.freeze) }
      end
      def gauge(name, help = '', labels = {})
        k = Observability.metric_key(name, labels)
        synchronize { @gauges[k] ||= Gauge.new(name, help, labels.dup.freeze) }
      end
      def histogram(name, help = '', labels = {}, bounds = nil)
        k = Observability.metric_key(name, labels)
        synchronize { @histograms[k] ||= Histogram.new(name, help, labels.dup.freeze, bounds) }
      end
      def counters; synchronize { @counters.values.sort_by(&:name) }; end
      def gauges; synchronize { @gauges.values.sort_by(&:name) }; end
      def histograms; synchronize { @histograms.values.sort_by(&:name) }; end
    end

    # --- Observability root ---

    Config = Struct.new(
      :slug, :default_log_level, :prom_addr, :redacted_fields,
      :logs_ring_size, :events_ring_size, :run_dir,
      :instance_uid, :organism_uid, :organism_slug,
      keyword_init: true
    ) do
      def initialize(**opts)
        super
        self.slug ||= ''
        self.default_log_level ||= LEVELS[:info]
        self.prom_addr ||= ''
        self.redacted_fields ||= []
        self.logs_ring_size ||= 1024
        self.events_ring_size ||= 256
        self.run_dir ||= ''
        self.instance_uid ||= ''
        self.organism_uid ||= ''
        self.organism_slug ||= ''
      end
    end

    class Logger
      attr_reader :name
      def initialize(obs, name)
        @obs = obs; @name = name
        @level = obs.cfg.default_log_level
        @mu = Monitor.new
      end
      def set_level(l); @mu.synchronize { @level = l }; end
      def enabled?(l); @obs && l >= @mu.synchronize { @level }; end

      def log(lvl, message, fields = nil)
        return unless enabled?(lvl)
        redact = @obs.cfg.redacted_fields.to_set
        out = {}
        (fields || {}).each do |k, v|
          next if k.nil? || k.to_s.empty?
          out[k.to_s] = redact.include?(k.to_s) ? '<redacted>' : v.to_s
        end
        entry = {
          timestamp: Time.now,
          level: lvl,
          slug: @obs.cfg.slug,
          instance_uid: @obs.cfg.instance_uid,
          session_id: '',
          rpc_method: '',
          message: message,
          fields: out,
          caller: '',
          chain: []
        }
        @obs.log_ring&.push(entry)
      end

      %i[trace debug info warn error fatal].each do |m|
        define_method(m) { |msg, fields = nil| log(LEVELS[m], msg, fields) }
      end
    end

    class Instance
      attr_reader :cfg, :families, :log_ring, :event_bus, :registry
      def initialize(cfg, families)
        @cfg = cfg
        @families = families
        @log_ring = families.include?(:logs) ? LogRing.new(cfg.logs_ring_size) : nil
        @event_bus = families.include?(:events) ? EventBus.new(cfg.events_ring_size) : nil
        @registry = families.include?(:metrics) ? Registry.new : nil
        @loggers = {}
        @mu = Monitor.new
      end

      def enabled?(family); @families.include?(family); end

      def organism_root?
        !@cfg.organism_uid.empty? && @cfg.organism_uid == @cfg.instance_uid
      end

      def logger(name)
        return DISABLED_LOGGER unless @families.include?(:logs)
        @mu.synchronize { @loggers[name] ||= Logger.new(self, name) }
      end

      def counter(name, help = '', labels = {}); @registry&.counter(name, help, labels); end
      def gauge(name, help = '', labels = {}); @registry&.gauge(name, help, labels); end
      def histogram(name, help = '', labels = {}, bounds = nil); @registry&.histogram(name, help, labels, bounds); end

      def emit(type, payload = nil)
        return unless @event_bus
        redact = @cfg.redacted_fields.to_set
        p = {}
        (payload || {}).each { |k, v| p[k.to_s] = redact.include?(k.to_s) ? '<redacted>' : v.to_s }
        @event_bus.emit({
          timestamp: Time.now, type: type, slug: @cfg.slug,
          instance_uid: @cfg.instance_uid, session_id: '', payload: p, chain: []
        })
      end

      def close; @event_bus&.close; end
    end

    DISABLED_LOGGER = Logger.new(
      Instance.new(Config.new, Set.new),
      ''
    )

    @current = nil
    @mu = Monitor.new

    def self.configure(cfg = Config.new)
      families = parse_op_obs(ENV['OP_OBS'])
      cfg.slug = File.basename($PROGRAM_NAME) if cfg.slug.nil? || cfg.slug.empty?
      inst = Instance.new(cfg, families)
      @mu.synchronize { @current = inst }
      inst
    end

    def self.from_env(base = Config.new)
      base.instance_uid = ENV['OP_INSTANCE_UID'] || '' if base.instance_uid.empty?
      base.organism_uid = ENV['OP_ORGANISM_UID'] || '' if base.organism_uid.empty?
      base.organism_slug = ENV['OP_ORGANISM_SLUG'] || '' if base.organism_slug.empty?
      base.prom_addr = ENV['OP_PROM_ADDR'] || '' if base.prom_addr.empty?
      base.run_dir = ENV['OP_RUN_DIR'] || '' if base.run_dir.empty?
      configure(base)
    end

    def self.current
      @mu.synchronize { @current } || Instance.new(Config.new, Set.new)
    end

    def self.reset
      @mu.synchronize do
        @current&.close
        @current = nil
      end
    end

    # --- Disk writers ---

    def self.enable_disk_writers(run_dir)
      inst = current
      return if run_dir.nil? || run_dir.empty?
      FileUtils.mkdir_p(run_dir)

      if inst.enabled?(:logs) && inst.log_ring
        fp = File.join(run_dir, 'stdout.log')
        inst.log_ring.subscribe { |e| append_log(fp, e) }
      end
      if inst.enabled?(:events) && inst.event_bus
        fp = File.join(run_dir, 'events.jsonl')
        inst.event_bus.subscribe { |e| append_event(fp, e) }
      end
    end

    def self.append_log(fp, e)
      rec = {
        kind: 'log',
        ts: e[:timestamp].utc.iso8601(9),
        level: LEVEL_LABELS[e[:level]],
        slug: e[:slug],
        instance_uid: e[:instance_uid],
        message: e[:message]
      }
      rec[:session_id] = e[:session_id] unless e[:session_id].empty?
      rec[:rpc_method] = e[:rpc_method] unless e[:rpc_method].empty?
      rec[:fields] = e[:fields] unless e[:fields].empty?
      rec[:caller] = e[:caller] unless e[:caller].empty?
      rec[:chain] = e[:chain] unless e[:chain].empty?
      File.open(fp, 'a') { |f| f.puts(JSON.generate(rec)) } rescue nil
    end

    def self.append_event(fp, e)
      rec = {
        kind: 'event',
        ts: e[:timestamp].utc.iso8601(9),
        type: EVENT_TYPE_LABELS[e[:type]],
        slug: e[:slug],
        instance_uid: e[:instance_uid]
      }
      rec[:session_id] = e[:session_id] unless e[:session_id].empty?
      rec[:payload] = e[:payload] unless e[:payload].empty?
      rec[:chain] = e[:chain] unless e[:chain].empty?
      File.open(fp, 'a') { |f| f.puts(JSON.generate(rec)) } rescue nil
    end

    # --- MetaJson ---

    MetaJson = Struct.new(
      :slug, :uid, :pid, :started_at, :mode, :transport, :address,
      :metrics_addr, :log_path, :log_bytes_rotated,
      :organism_uid, :organism_slug, :is_default,
      keyword_init: true
    )

    def self.write_meta_json(run_dir, meta)
      FileUtils.mkdir_p(run_dir)
      dict = {
        slug: meta.slug, uid: meta.uid, pid: meta.pid,
        started_at: meta.started_at.utc.iso8601(9),
        mode: meta.mode, transport: meta.transport, address: meta.address
      }
      dict[:metrics_addr] = meta.metrics_addr unless meta.metrics_addr.to_s.empty?
      dict[:log_path] = meta.log_path unless meta.log_path.to_s.empty?
      dict[:log_bytes_rotated] = meta.log_bytes_rotated if meta.log_bytes_rotated.to_i > 0
      dict[:organism_uid] = meta.organism_uid unless meta.organism_uid.to_s.empty?
      dict[:organism_slug] = meta.organism_slug unless meta.organism_slug.to_s.empty?
      dict[:default] = true if meta.is_default
      path = File.join(run_dir, 'meta.json')
      tmp = path + '.tmp'
      File.write(tmp, JSON.pretty_generate(dict))
      File.rename(tmp, path)
    end
  end
end
