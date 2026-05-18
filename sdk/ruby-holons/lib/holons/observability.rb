# Ruby reference implementation of the cross-SDK observability layer.
# Mirrors sdk/go-holons/pkg/observability. See OBSERVABILITY.md.

require 'fileutils'
require 'json'
require 'monitor'
require 'securerandom'
require 'set'
require 'thread'
require 'time'
require 'webrick'

module Holons
  module Observability
    FAMILIES = {
      logs: :logs, metrics: :metrics, events: :events, prom: :prom
    }.freeze

    V1_TOKENS = %w[logs metrics events prom all].to_set.freeze

    class InvalidTokenError < StandardError
      attr_reader :token
      def initialize(token, reason)
        @token = token
        super("OP_OBS: #{reason}: #{token}")
      end
    end

    LEVELS = { trace: 1, debug: 5, info: 9, warn: 13, error: 17, fatal: 21 }.freeze
    LEVEL_LABELS = {
      1 => 'TRACE', 5 => 'DEBUG', 9 => 'INFO',
      13 => 'WARN', 17 => 'ERROR', 21 => 'FATAL'
    }.freeze
    SEVERITY_NUMBERS = {
      SEVERITY_NUMBER_UNSPECIFIED: 0,
      SEVERITY_NUMBER_TRACE: 1,
      SEVERITY_NUMBER_DEBUG: 5,
      SEVERITY_NUMBER_INFO: 9,
      SEVERITY_NUMBER_WARN: 13,
      SEVERITY_NUMBER_ERROR: 17,
      SEVERITY_NUMBER_FATAL: 21
    }.freeze

    EVENT_INSTANCE_SPAWNED = 'instance.spawned'
    EVENT_INSTANCE_READY = 'instance.ready'
    EVENT_INSTANCE_EXITED = 'instance.exited'
    EVENT_INSTANCE_CRASHED = 'instance.crashed'
    EVENT_SESSION_STARTED = 'session.started'
    EVENT_SESSION_ENDED = 'session.ended'
    EVENT_HANDLER_PANIC = 'handler.panic'
    EVENT_CONFIG_RELOADED = 'config.reloaded'
    EVENT_NAMES = {
      instance_spawned: EVENT_INSTANCE_SPAWNED,
      instance_ready: EVENT_INSTANCE_READY,
      instance_exited: EVENT_INSTANCE_EXITED,
      instance_crashed: EVENT_INSTANCE_CRASHED,
      session_started: EVENT_SESSION_STARTED,
      session_ended: EVENT_SESSION_ENDED,
      handler_panic: EVENT_HANDLER_PANIC,
      config_reloaded: EVENT_CONFIG_RELOADED
    }.freeze
    CANONICAL_EVENT_NAMES = EVENT_NAMES.values.to_set.freeze

    ATTR_HOLONS_SLUG = 'holons.slug'
    ATTR_HOLONS_INSTANCE_UID = 'holons.instance_uid'
    ATTR_HOLONS_SESSION_ID = 'holons.session_id'
    ATTR_SERVICE_NAME = 'service.name'
    ATTR_SERVICE_INSTANCE_ID = 'service.instance.id'
    ATTR_RPC_METHOD = 'rpc.method'
    ATTR_LOGGER_NAME = 'logger.name'
    ATTR_CODE_CALLER = 'code.caller'

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
        raise InvalidTokenError.new(tok, 'unknown OP_OBS token') unless V1_TOKENS.include?(tok)
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
        raise InvalidTokenError.new(tok, 'unknown OP_OBS token') unless V1_TOKENS.include?(tok)
      end
    end

    def self.append_direct_child(src, child_slug)
      base = src.respond_to?(:to_a) ? src.to_a : Array(src)
      out = base.map(&:to_s)
      slug = child_slug.to_s
      out << slug unless slug.empty?
      out
    end

    def self.enrich_for_multilog(wire, src_slug)
      append_direct_child(wire, src_slug)
    end

    LogRecordEnvelope = Struct.new(:record, :private, keyword_init: true) do
      def timestamp
        return Time.at(0) if record.nil? || record.time_unix_nano.to_i.zero?

        Time.at(0, record.time_unix_nano.to_i, :nsec)
      end
    end

    ContextValues = Struct.new(:session_id, :rpc_method, keyword_init: true) do
      def initialize(**opts)
        super
        self.session_id ||= ''
        self.rpc_method ||= ''
      end
    end
    CONTEXT_KEY = :holons_observability_context

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
      def drain_since(cutoff); synchronize { @buf.select { |e| e.timestamp >= cutoff } }; end
      def size; synchronize { @buf.size }; end

      def subscribe(&fn)
        synchronize { @subs << fn }
        -> { synchronize { @subs.delete(fn) } }
      end

      def replay_and_watch(cutoff = nil, &fn)
        synchronize do
          # Snapshot and subscription registration are one critical section:
          # follow=true streams must not drop entries at the replay/live seam.
          replay = cutoff.nil? ? @buf.dup : @buf.select { |e| e.timestamp >= cutoff }
          @subs << fn
          [replay, -> { synchronize { @subs.delete(fn) } }]
        end
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
      def drain_since(cutoff); synchronize { @buf.select { |e| e.timestamp >= cutoff } }; end

      def subscribe(&fn)
        synchronize { @subs << fn }
        -> { synchronize { @subs.delete(fn) } }
      end

      def replay_and_watch(cutoff = nil, &fn)
        synchronize do
          # Snapshot and subscription registration are one critical section:
          # follow=true streams must not drop events at the replay/live seam.
          replay = cutoff.nil? ? @buf.dup : @buf.select { |e| e.timestamp >= cutoff }
          @subs << fn
          [replay, -> { synchronize { @subs.delete(fn) } }]
        end
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
      attr_reader :bounds, :counts, :total, :sum, :min, :max
      def initialize(bounds, counts, total, sum, min, max)
        @bounds = bounds; @counts = counts; @total = total; @sum = sum
        @min = min; @max = max
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
        @min = 0.0
        @max = 0.0
      end

      def observe(v)
        value = v.to_f
        synchronize do
          @min = value if @total.zero? || value < @min
          @max = value if @total.zero? || value > @max
          @total += 1
          @sum += value
          @bounds.each_with_index { |b, i| @counts[i] += 1 if value <= b }
        end
      end

      def observe_duration(seconds); observe(seconds); end

      def snapshot
        synchronize { HistogramSnapshot.new(@bounds.dup, @counts.dup, @total, @sum, @min, @max) }
      end
    end

    def self.metric_key(name, labels)
      return name if labels.nil? || labels.empty?
      "#{name}|" + labels.sort_by { |k, _| k.to_s }.map { |k, v| "#{k}=#{v}" }.join(',')
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
      :instance_uid, :session_id, :organism_uid, :organism_slug,
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
        self.session_id ||= ''
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

      def log(lvl, message, fields = nil, private: false, **keyword_fields)
        return unless enabled?(lvl)

        redact = @obs.cfg.redacted_fields.to_set
        out = {}
        merged_fields = {}
        (fields || {}).each { |k, v| merged_fields[k] = v }
        keyword_fields.each { |k, v| merged_fields[k] = v }
        merged_fields.each do |k, v|
          next if k.nil? || k.to_s.empty?
          key = k.to_s
          out[key] = redact.include?(key) ? '<redacted>' : v
        end
        ctx = Observability.current_context
        session_id = ctx.session_id.empty? ? @obs.cfg.session_id : ctx.session_id
        record = Observability.build_log_record(
          slug: @obs.cfg.slug,
          instance_uid: @obs.cfg.instance_uid,
          logger_name: @name,
          severity_number: lvl,
          body: message.to_s,
          attributes: out,
          session_id: session_id,
          rpc_method: ctx.rpc_method
        )
        @obs.log_ring&.push(LogRecordEnvelope.new(record: record, private: private ? true : false))
      end

      %i[trace debug info warn error fatal].each do |m|
        define_method(m) { |msg, fields = nil, private: false, **keyword_fields| log(LEVELS[m], msg, fields, private: private, **keyword_fields) }
      end
    end

    class Instance
      attr_reader :cfg, :families, :log_ring, :event_bus, :registry, :start_wall
      def initialize(cfg, families)
        @cfg = cfg
        @families = families
        @log_ring = families.include?(:logs) ? LogRing.new(cfg.logs_ring_size) : nil
        @event_bus = families.include?(:events) ? EventBus.new(cfg.events_ring_size) : nil
        @registry = families.include?(:metrics) ? Registry.new : nil
        @start_wall = Time.now
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

      def emit(event_name, payload = nil, private: false, **keyword_payload)
        return unless @event_bus

        redact = @cfg.redacted_fields.to_set
        p = {}
        merged_payload = {}
        (payload || {}).each { |k, v| merged_payload[k] = v }
        keyword_payload.each { |k, v| merged_payload[k] = v }
        merged_payload.each do |k, v|
          key = k.to_s
          p[key] = redact.include?(key) ? '<redacted>' : v
        end
        canonical_name = Observability.canonical_event_name(event_name)
        ctx = Observability.current_context
        session_id = ctx.session_id.empty? ? @cfg.session_id : ctx.session_id
        record = Observability.build_event_record(
          slug: @cfg.slug,
          instance_uid: @cfg.instance_uid,
          event_name: canonical_name,
          attributes: p,
          session_id: session_id
        )
        @event_bus.emit(LogRecordEnvelope.new(record: record, private: private ? true : false))
      end

      def close; @event_bus&.close; end
    end

    DISABLED_LOGGER = Logger.new(
      Instance.new(Config.new, Set.new),
      ''
    )

    @current = nil
    @mu = Monitor.new

    def self.configure(cfg = Config.new, env: ENV)
      check_env(env)
      families = parse_op_obs(env['OP_OBS'])
      cfg.slug = File.basename($PROGRAM_NAME) if cfg.slug.nil? || cfg.slug.empty?
      cfg.instance_uid = SecureRandom.uuid if cfg.instance_uid.nil? || cfg.instance_uid.empty?
      cfg.session_id = SecureRandom.uuid if cfg.session_id.nil? || cfg.session_id.empty?
      cfg.run_dir = derive_run_dir(cfg.run_dir, cfg.slug, cfg.instance_uid) unless cfg.run_dir.to_s.empty?
      inst = Instance.new(cfg, families)
      @mu.synchronize { @current = inst }
      inst
    end

    def self.from_env(base = Config.new, env: ENV)
      base.instance_uid = env['OP_INSTANCE_UID'] || '' if base.instance_uid.empty?
      base.organism_uid = env['OP_ORGANISM_UID'] || '' if base.organism_uid.empty?
      base.organism_slug = env['OP_ORGANISM_SLUG'] || '' if base.organism_slug.empty?
      base.prom_addr = env['OP_PROM_ADDR'] || '' if base.prom_addr.empty?
      base.run_dir = env['OP_RUN_DIR'] || '' if base.run_dir.empty?
      configure(base, env: env)
    end

    def self.current
      @mu.synchronize { @current } || Instance.new(Config.new, Set.new)
    end

    def self.current_context
      Thread.current[CONTEXT_KEY] || ContextValues.new
    end

    def self.with_context(session_id: '', rpc_method: '')
      previous = Thread.current[CONTEXT_KEY]
      Thread.current[CONTEXT_KEY] = ContextValues.new(session_id: session_id.to_s, rpc_method: rpc_method.to_s)
      yield
    ensure
      Thread.current[CONTEXT_KEY] = previous
    end

    def self.reset
      @mu.synchronize do
        @current&.close
        @current = nil
      end
    end

    def self.derive_run_dir(root, slug, uid)
      return root if root.to_s.empty? || slug.to_s.empty? || uid.to_s.empty?

      File.join(root, slug, uid)
    end

    def self.register_grpc_service(server, inst = current)
      raise ArgumentError, 'grpc server is required' if server.nil?

      require_grpc_observability_support!
      server.handle(observability_service_class.new(inst))
    end

    def self.observability_service_class
      return @observability_service_class if defined?(@observability_service_class) && !@observability_service_class.nil?

      require_grpc_observability_support!
      @observability_service_class = Class.new(::Holons::V1::HolonObservability::Service) do
        def initialize(inst)
          @inst = inst
        end

        def logs(request, call)
          raise ::GRPC::FailedPrecondition, 'logs family is not enabled (OP_OBS)' unless @inst.enabled?(:logs) && @inst.log_ring

          Enumerator.new do |y|
            q = Queue.new
            cutoff = request.since.nil? ? nil : Time.now - Holons::Observability.duration_seconds(request.since)
            entries = nil
            unsubscribe = nil
            if request.follow
              entries, unsubscribe = @inst.log_ring.replay_and_watch(cutoff) { |entry| q << entry }
            else
              entries = cutoff.nil? ? @inst.log_ring.drain : @inst.log_ring.drain_since(cutoff)
            end
            Holons::Observability.write_matching_logs(y, entries, request)
            next unless request.follow

            begin
              loop do
                break if call.respond_to?(:cancelled?) && call.cancelled?

                entry = Holons::Observability.queue_pop(q)
                next if entry.nil?

                Holons::Observability.write_matching_logs(y, [entry], request)
              end
            ensure
              unsubscribe&.call
            end
          end
        end

        def metrics(request, _call)
          raise ::GRPC::FailedPrecondition, 'metrics family is not enabled (OP_OBS)' unless @inst.enabled?(:metrics) && @inst.registry

          Enumerator.new do |y|
            metrics = Holons::Observability.to_proto_metrics(
              @inst.registry,
              @inst.cfg.slug,
              @inst.cfg.instance_uid,
              @inst.start_wall
            )
            metrics.each do |metric|
              next unless request.name_prefixes.empty? || request.name_prefixes.any? { |prefix| metric.name.start_with?(prefix) }

              y << metric
            end
          end
        end

        def events(request, call)
          raise ::GRPC::FailedPrecondition, 'events family is not enabled (OP_OBS)' unless @inst.enabled?(:events) && @inst.event_bus

          Enumerator.new do |y|
            q = Queue.new
            cutoff = request.since.nil? ? nil : Time.now - Holons::Observability.duration_seconds(request.since)
            events = nil
            unsubscribe = nil
            if request.follow
              events, unsubscribe = @inst.event_bus.replay_and_watch(cutoff) { |event| q << event }
            else
              events = cutoff.nil? ? @inst.event_bus.drain : @inst.event_bus.drain_since(cutoff)
            end
            Holons::Observability.write_matching_events(y, events, request)
            next unless request.follow

            begin
              loop do
                break if call.respond_to?(:cancelled?) && call.cancelled?

                event = Holons::Observability.queue_pop(q)
                next if event.nil?

                Holons::Observability.write_matching_events(y, [event], request)
              end
            ensure
              unsubscribe&.call
            end
          end
        end
      end
    end

    def self.write_matching_logs(stream, entries, request)
      min_level = severity_number(request.min_severity_number)
      min_level = LEVELS[:info] if min_level.zero?
      entries.each do |entry|
        record = entry.record
        next if record.nil?
        next if entry.private
        next if severity_number(record.severity_number) < min_level
        next if !request.session_ids.empty? && !request.session_ids.include?(attribute_string(record.attributes, ATTR_HOLONS_SESSION_ID))
        next if !request.rpc_methods.empty? && !request.rpc_methods.include?(attribute_string(record.attributes, ATTR_RPC_METHOD))

        stream << to_proto_log_record(entry)
      end
    end

    def self.write_matching_events(stream, events, request)
      wanted = request.event_names.to_set
      events.each do |event|
        record = event.record
        next if record.nil?
        next if event.private
        next if !wanted.empty? && !wanted.include?(record.event_name)

        stream << to_proto_log_record(event)
      end
    end

    def self.queue_pop(queue)
      queue.pop(true)
    rescue ThreadError
      sleep 0.05
      nil
    end

    def self.to_proto_log_record(entry)
      clone_log_record(entry.record)
    end

    def self.from_proto_log_record(record)
      LogRecordEnvelope.new(record: clone_log_record(record), private: false)
    end

    def self.clone_log_record(record)
      require_observability_proto!
      return ::Holons::V1::LogRecord.new if record.nil?

      ::Holons::V1::LogRecord.decode(::Holons::V1::LogRecord.encode(record))
    end

    def self.build_log_record(slug:, instance_uid:, logger_name:, severity_number:, body:, attributes: {}, session_id: '', rpc_method: '', chain: [])
      require_observability_proto!
      now = Time.now
      attrs = resource_attributes(slug, instance_uid)
      attrs << key_value(ATTR_LOGGER_NAME, logger_name) unless logger_name.to_s.empty?
      attrs << key_value(ATTR_HOLONS_SESSION_ID, session_id) unless session_id.to_s.empty?
      attrs << key_value(ATTR_RPC_METHOD, rpc_method) unless rpc_method.to_s.empty?
      attributes.each { |key, value| attrs << key_value(key, value) unless key.to_s.empty? }
      ::Holons::V1::LogRecord.new(
        time_unix_nano: time_unix_nano(now),
        observed_time_unix_nano: time_unix_nano(now),
        severity_number: severity_number,
        severity_text: severity_text(severity_number),
        body: to_any_value(body.to_s),
        attributes: attrs,
        chain: Array(chain).map(&:to_s)
      )
    end

    def self.build_event_record(slug:, instance_uid:, event_name:, attributes: {}, session_id: '', chain: [])
      require_observability_proto!
      now = Time.now
      attrs = resource_attributes(slug, instance_uid)
      attrs << key_value(ATTR_HOLONS_SESSION_ID, session_id) unless session_id.to_s.empty?
      attributes.each { |key, value| attrs << key_value(key, value) unless key.to_s.empty? }
      ::Holons::V1::LogRecord.new(
        time_unix_nano: time_unix_nano(now),
        observed_time_unix_nano: time_unix_nano(now),
        severity_number: LEVELS[:info],
        severity_text: LEVEL_LABELS.fetch(LEVELS[:info]),
        body: to_any_value(event_name),
        attributes: attrs,
        event_name: event_name,
        chain: Array(chain).map(&:to_s)
      )
    end

    def self.to_proto_metrics(registry, slug = '', instance_uid = '', start_time = Time.now)
      require_observability_proto!
      metrics = []
      start_nano = time_unix_nano(start_time)
      now_nano = time_unix_nano(Time.now)
      registry.counters.each do |counter|
        metrics << ::Holons::V1::Metric.new(
          name: counter.name,
          description: counter.help,
          sum: ::Holons::V1::Sum.new(
            aggregation_temporality: :AGGREGATION_TEMPORALITY_CUMULATIVE,
            is_monotonic: true,
            data_points: [
              ::Holons::V1::NumberDataPoint.new(
                start_time_unix_nano: start_nano,
                time_unix_nano: now_nano,
                as_int: counter.value,
                attributes: metric_attributes(slug, instance_uid, counter.labels)
              )
            ]
          )
        )
      end
      registry.gauges.each do |gauge|
        metrics << ::Holons::V1::Metric.new(
          name: gauge.name,
          description: gauge.help,
          gauge: ::Holons::V1::Gauge.new(
            data_points: [
              ::Holons::V1::NumberDataPoint.new(
                start_time_unix_nano: start_nano,
                time_unix_nano: now_nano,
                as_double: gauge.value,
                attributes: metric_attributes(slug, instance_uid, gauge.labels)
              )
            ]
          )
        )
      end
      registry.histograms.each do |histogram|
        snapshot = histogram.snapshot
        metrics << ::Holons::V1::Metric.new(
          name: histogram.name,
          description: histogram.help,
          histogram: ::Holons::V1::Histogram.new(
            aggregation_temporality: :AGGREGATION_TEMPORALITY_CUMULATIVE,
            data_points: [
              ::Holons::V1::HistogramDataPoint.new(
                start_time_unix_nano: start_nano,
                time_unix_nano: now_nano,
                attributes: metric_attributes(slug, instance_uid, histogram.labels),
                count: snapshot.total,
                sum: snapshot.sum,
                bucket_counts: histogram_bucket_counts(snapshot),
                explicit_bounds: snapshot.bounds,
                min: snapshot.min,
                max: snapshot.max
              )
            ]
          )
        )
      end
      metrics
    end

    def self.histogram_bucket_counts(snapshot)
      counts = []
      previous = 0
      snapshot.counts.each do |count|
        delta = count - previous
        counts << [delta, 0].max
        previous = count
      end
      counts << [snapshot.total - previous, 0].max
      counts
    end

    def self.metric_attributes(slug, instance_uid, labels)
      resource_attributes(slug, instance_uid) + sorted_attributes(labels || {})
    end

    def self.resource_attributes(slug, instance_uid)
      attrs = []
      unless slug.to_s.empty?
        attrs << key_value(ATTR_HOLONS_SLUG, slug)
        attrs << key_value(ATTR_SERVICE_NAME, slug)
      end
      unless instance_uid.to_s.empty?
        attrs << key_value(ATTR_HOLONS_INSTANCE_UID, instance_uid)
        attrs << key_value(ATTR_SERVICE_INSTANCE_ID, instance_uid)
      end
      attrs
    end

    def self.sorted_attributes(labels)
      labels.keys.map(&:to_s).sort.map do |key|
        value = labels.key?(key) ? labels[key] : labels[key.to_sym]
        key_value(key, value)
      end
    end

    def self.key_value(key, value)
      require_observability_proto!
      ::Holons::V1::KeyValue.new(key: key.to_s, value: to_any_value(value))
    end

    def self.to_any_value(value)
      require_observability_proto!
      case value
      when TrueClass, FalseClass
        ::Holons::V1::AnyValue.new(bool_value: value)
      when Integer
        ::Holons::V1::AnyValue.new(int_value: value)
      when Float
        ::Holons::V1::AnyValue.new(double_value: value)
      when String, Symbol
        ::Holons::V1::AnyValue.new(string_value: value.to_s)
      else
        ::Holons::V1::AnyValue.new(string_value: value.to_s)
      end
    end

    def self.any_value_to_ruby(value)
      return '' if value.nil?

      case value.value
      when :bool_value
        value.bool_value
      when :int_value
        value.int_value
      when :double_value
        value.double_value
      when :string_value
        value.string_value
      else
        ''
      end
    end

    def self.any_value_string(value)
      any_value_to_ruby(value).to_s
    end

    def self.attribute_value(attributes, key)
      attributes.find { |attr| attr.key == key.to_s }&.value
    end

    def self.attribute_string(attributes, key)
      any_value_string(attribute_value(attributes, key))
    end

    def self.attributes_hash(attributes, include_system: false)
      attributes.each_with_object({}) do |attr, out|
        next if attr.nil?
        next if !include_system && system_attribute?(attr.key)

        out[attr.key] = any_value_to_ruby(attr.value)
      end
    end

    def self.set_chain(record, chain)
      record.chain.clear
      Array(chain).each { |hop| record.chain << hop.to_s }
      record
    end

    def self.system_attribute?(key)
      [
        ATTR_HOLONS_SLUG, ATTR_HOLONS_INSTANCE_UID, ATTR_HOLONS_SESSION_ID,
        ATTR_SERVICE_NAME, ATTR_SERVICE_INSTANCE_ID, ATTR_RPC_METHOD,
        ATTR_LOGGER_NAME, ATTR_CODE_CALLER
      ].include?(key.to_s)
    end

    def self.body_string(record)
      any_value_string(record&.body)
    end

    def self.record_slug(record)
      return '' if record.nil?

      attribute_string(record.attributes, ATTR_HOLONS_SLUG)
    end

    def self.record_instance_uid(record)
      return '' if record.nil?

      attribute_string(record.attributes, ATTR_HOLONS_INSTANCE_UID)
    end

    def self.time_unix_nano(time)
      (time.to_r * 1_000_000_000).to_i
    end

    def self.duration_seconds(duration)
      duration.seconds.to_f + (duration.nanos.to_f / 1_000_000_000.0)
    end

    def self.severity_number(value)
      return value.to_i unless value.is_a?(Symbol)

      SEVERITY_NUMBERS.fetch(value, 0)
    end

    def self.severity_text(value)
      LEVEL_LABELS.fetch(severity_number(value), 'UNSPECIFIED')
    end

    def self.canonical_event_name(value)
      name = value.is_a?(Symbol) ? EVENT_NAMES.fetch(value, '') : value.to_s
      raise ArgumentError, "unknown observability event_name: #{value}" unless CANONICAL_EVENT_NAMES.include?(name)

      name
    end

    # --- Prometheus exposition ---

    class PromServer
      def initialize(addr = ':0')
        @addr = addr.to_s.empty? ? ':0' : addr.to_s
        @server = nil
        @thread = nil
        @mu = Monitor.new
      end

      def start
        @mu.synchronize do
          return addr_url_unlocked unless @server.nil?

          host, port = Holons::Observability.parse_prom_addr(@addr)
          logger = WEBrick::Log.new(File::NULL)
          @server = WEBrick::HTTPServer.new(
            BindAddress: host,
            Port: port,
            Logger: logger,
            AccessLog: [],
            StartCallback: nil
          )
          @server.mount_proc('/metrics') { |_request, response| write_metrics_response(response) }
          @thread = Thread.new { @server.start }
          addr_url_unlocked
        end
      end

      def close
        server = nil
        thread = nil
        @mu.synchronize do
          server = @server
          thread = @thread
          @server = nil
          @thread = nil
        end
        return if server.nil?

        server.shutdown
        thread&.join(1)
      rescue StandardError
        nil
      end

      private

      def write_metrics_response(response)
        obs = Holons::Observability.current
        response['Content-Type'] = 'text/plain; version=0.0.4'
        if !obs.enabled?(:metrics)
          response.status = 503
          response.body = "# metrics family disabled (OP_OBS)\n"
        elsif !obs.enabled?(:prom)
          response.status = 503
          response.body = "# prom family disabled (OP_OBS)\n"
        else
          response.status = 200
          response.body = Holons::Observability.to_prometheus_text(obs)
        end
      end

      def addr_url_unlocked
        return '' if @server.nil? || @server.listeners.empty?

        addr = @server.listeners.first.addr
        port = addr[1]
        host = Holons::Observability.advertised_prom_host(addr[3] || addr[2].to_s)
        "http://#{host}:#{port}/metrics"
      end
    end

    def self.to_prometheus_text(obs)
      return "# metrics family disabled (OP_OBS)\n" unless obs.enabled?(:metrics) && obs.registry

      groups = {}
      ensure_group = lambda do |name, help, type|
        groups[name] ||= { name: name, help: help, type: type, counters: [], gauges: [], histograms: [] }
        groups[name][:help] = help if groups[name][:help].to_s.empty? && !help.to_s.empty?
        groups[name]
      end

      obs.registry.counters.each { |counter| ensure_group.call(counter.name, counter.help, 'counter')[:counters] << counter }
      obs.registry.gauges.each { |gauge| ensure_group.call(gauge.name, gauge.help, 'gauge')[:gauges] << gauge }
      obs.registry.histograms.each { |histogram| ensure_group.call(histogram.name, histogram.help, 'histogram')[:histograms] << histogram }

      injected = { 'slug' => obs.cfg.slug.to_s }
      injected['instance_uid'] = obs.cfg.instance_uid.to_s unless obs.cfg.instance_uid.to_s.empty?

      lines = []
      groups.keys.sort.each do |name|
        group = groups[name]
        lines << "# HELP #{name} #{prom_escape_help(group[:help].to_s)}"
        lines << "# TYPE #{name} #{group[:type]}"
        group[:counters].each do |counter|
          lines << "#{counter.name}#{prom_labels(merge_prom_labels(counter.labels, injected))} #{counter.value}"
        end
        group[:gauges].each do |gauge|
          lines << "#{gauge.name}#{prom_labels(merge_prom_labels(gauge.labels, injected))} #{format_prom_float(gauge.value)}"
        end
        group[:histograms].each do |histogram|
          labels = merge_prom_labels(histogram.labels, injected)
          snapshot = histogram.snapshot
          snapshot.bounds.each_with_index do |bound, idx|
            bucket_labels = labels.merge('le' => format_prom_float(bound))
            lines << "#{histogram.name}_bucket#{prom_labels(bucket_labels)} #{snapshot.counts[idx]}"
          end
          lines << "#{histogram.name}_bucket#{prom_labels(labels.merge('le' => '+Inf'))} #{snapshot.total}"
          lines << "#{histogram.name}_sum#{prom_labels(labels)} #{format_prom_float(snapshot.sum)}"
          lines << "#{histogram.name}_count#{prom_labels(labels)} #{snapshot.total}"
        end
      end
      lines.empty? ? '' : "#{lines.join("\n")}\n"
    end

    def self.parse_prom_addr(raw)
      trimmed = raw.to_s.strip
      trimmed = ':0' if trimmed.empty?
      return ['0.0.0.0', Integer(trimmed[1..] || '0', 10)] if trimmed.start_with?(':')

      host, separator, port_text = trimmed.rpartition(':')
      raise ArgumentError, %(invalid Prometheus address "#{raw}") if separator.empty? || port_text.empty?

      [host.empty? ? '0.0.0.0' : host, Integer(port_text, 10)]
    end

    def self.advertised_prom_host(host)
      case host.to_s
      when '', '0.0.0.0'
        '127.0.0.1'
      when '::'
        '::1'
      else
        host.to_s
      end
    end

    def self.merge_prom_labels(base, extra)
      out = {}
      extra.each { |key, value| out[key.to_s] = value.to_s unless value.to_s.empty? }
      (base || {}).each { |key, value| out[key.to_s] = value.to_s }
      out
    end

    def self.prom_labels(labels)
      return '' if labels.nil? || labels.empty?

      '{' + labels.keys.sort.map { |key| %(#{key}="#{prom_escape_value(labels[key])}") }.join(',') + '}'
    end

    def self.prom_escape_value(value)
      value.to_s.gsub('\\', '\\\\\\').gsub("\n", '\\n').gsub('"', '\\"')
    end

    def self.prom_escape_help(value)
      value.to_s.gsub('\\', '\\\\\\').gsub("\n", '\\n')
    end

    def self.format_prom_float(value)
      number = value.to_f
      return '+Inf' if number.infinite? == 1
      return '-Inf' if number.infinite? == -1
      return 'NaN' if number.nan?

      format('%g', number)
    end

    # --- Member observability relay ---

    class MemberRelay
      def initialize(child_slug:, child_uid:, stub:, observability: nil, retry_delay: 2.0)
        @child_slug = child_slug.to_s
        @child_uid = child_uid.to_s
        @stub = stub
        @observability = observability || Holons::Observability.current
        @retry_delay = retry_delay.to_f
        @stop = false
        @threads = []
        @mu = Mutex.new
      end

      def start
        obs = @observability
        return unless obs.enabled?(:logs) || obs.enabled?(:events)

        if obs.enabled?(:logs) && obs.log_ring
          @threads << Thread.new { pump_logs }
        end
        if obs.enabled?(:events) && obs.event_bus
          @threads << Thread.new { pump_events }
        end
        @threads.each { |thread| thread.abort_on_exception = false }
        self
      end

      def stop
        @mu.synchronize { @stop = true }
        @threads.each { |thread| thread.join(0.2) }
        @threads.each do |thread|
          next unless thread.alive?

          thread.kill
          thread.join(0.2)
        end
      rescue StandardError
        nil
      end

      private

      def stopped?
        @mu.synchronize { @stop }
      end

      def pump_logs
        until stopped?
          begin
            @stub.logs(::Holons::V1::LogsRequest.new(follow: true)).each do |proto|
              return if stopped?

              obs = @observability
              next unless obs.enabled?(:logs) && obs.log_ring

              entry = Holons::Observability.from_proto_log_record(proto)
              Holons::Observability.set_chain(entry.record, Holons::Observability.append_direct_child(entry.record.chain, @child_slug))
              obs.log_ring.push(entry)
            end
          rescue StandardError => e
            warn("warning: observability relay events: #{e.message}") if ENV["HOLONS_DEBUG_RELAY"]
            sleep_retry
          end
        end
      end

      def pump_events
        until stopped?
          begin
            @stub.events(::Holons::V1::EventsRequest.new(follow: true)).each do |proto|
              return if stopped?

              obs = @observability
              next unless obs.enabled?(:events) && obs.event_bus

              event = Holons::Observability.from_proto_log_record(proto)
              Holons::Observability.set_chain(event.record, Holons::Observability.append_direct_child(event.record.chain, @child_slug))
              obs.event_bus.emit(event)
            end
          rescue StandardError
            sleep_retry
          end
        end
      end

      def sleep_retry
        deadline = Time.now + @retry_delay
        sleep 0.05 while !stopped? && Time.now < deadline
      end
    end

    def self.require_grpc_observability_support!
      return if defined?(@grpc_observability_loaded) && @grpc_observability_loaded

      require_observability_proto!
      require_relative '../gen/holons/v1/observability_services_pb'
      @grpc_observability_loaded = true
    end

    def self.require_observability_proto!
      return if defined?(@observability_proto_loaded) && @observability_proto_loaded

      ensure_generated_proto_load_path!
      require_relative '../gen/holons/v1/observability_pb'
      @observability_proto_loaded = true
    end

    def self.ensure_generated_proto_load_path!
      gen_root = File.expand_path('../gen', __dir__)
      $LOAD_PATH.unshift(gen_root) unless $LOAD_PATH.include?(gen_root)
    end

    # --- Disk writers ---

    def self.enable_disk_writers(run_dir)
      inst = current
      return if run_dir.nil? || run_dir.empty?
      FileUtils.mkdir_p(run_dir)

      if inst.enabled?(:logs) && inst.log_ring
        log_fp = File.join(run_dir, 'stdout.log')
        inst.log_ring.subscribe { |e| append_log(log_fp, e) }
      end
      if inst.enabled?(:events) && inst.event_bus
        event_fp = File.join(run_dir, 'events.jsonl')
        inst.event_bus.subscribe { |e| append_event(event_fp, e) }
      end
    end

    def self.append_log(fp, e)
      record = e.record
      rec = {
        kind: 'log',
        ts: e.timestamp.utc.iso8601(9),
        level: record.severity_text,
        slug: record_slug(record),
        instance_uid: record_instance_uid(record),
        message: body_string(record)
      }
      session_id = attribute_string(record.attributes, ATTR_HOLONS_SESSION_ID)
      rpc_method = attribute_string(record.attributes, ATTR_RPC_METHOD)
      caller = attribute_string(record.attributes, ATTR_CODE_CALLER)
      fields = attributes_hash(record.attributes)
      rec[:session_id] = session_id unless session_id.empty?
      rec[:rpc_method] = rpc_method unless rpc_method.empty?
      rec[:fields] = fields unless fields.empty?
      rec[:caller] = caller unless caller.empty?
      rec[:chain] = record.chain unless record.chain.empty?
      File.open(fp, 'a') { |f| f.puts(JSON.generate(rec)) } rescue nil
    end

    def self.append_event(fp, e)
      record = e.record
      rec = {
        kind: 'event',
        ts: e.timestamp.utc.iso8601(9),
        event_name: record.event_name,
        slug: record_slug(record),
        instance_uid: record_instance_uid(record)
      }
      session_id = attribute_string(record.attributes, ATTR_HOLONS_SESSION_ID)
      payload = attributes_hash(record.attributes)
      rec[:session_id] = session_id unless session_id.empty?
      rec[:payload] = payload unless payload.empty?
      rec[:chain] = record.chain unless record.chain.empty?
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
