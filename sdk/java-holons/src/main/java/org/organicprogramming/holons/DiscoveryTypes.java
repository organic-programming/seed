package org.organicprogramming.holons;

import java.util.ArrayList;
import java.util.List;

/** Shared types and constants for the uniform discovery/connect APIs. */
public final class DiscoveryTypes {
    public static final int LOCAL = 0;
    public static final int PROXY = 1;
    public static final int DELEGATED = 2;

    public static final int SIBLINGS = 0x01;
    public static final int CWD = 0x02;
    public static final int SOURCE = 0x04;
    public static final int BUILT = 0x08;
    public static final int INSTALLED = 0x10;
    public static final int CACHED = 0x20;
    public static final int ALL = 0x3F;

    public static final int NO_LIMIT = 0;
    public static final int NO_TIMEOUT = 0;

    private DiscoveryTypes() {
    }

    public static final class IdentityInfo {
        public String givenName = "";
        public String familyName = "";
        public String motto = "";
        public List<String> aliases = new ArrayList<>();
    }

    public static final class HolonInfo {
        public String slug = "";
        public String uuid = "";
        public IdentityInfo identity = new IdentityInfo();
        public String lang = "";
        public String runner = "";
        public String status = "";
        public String kind = "";
        public String transport = "";
        public String entrypoint = "";
        public List<String> architectures = new ArrayList<>();
        public boolean hasDist;
        public boolean hasSource;
    }

    public static final class HolonRef {
        public String url = "";
        public HolonInfo info;
        public String error = "";
    }

    public static final class DiscoverResult {
        public List<HolonRef> found = new ArrayList<>();
        public String error = "";
    }

    public static final class ResolveResult {
        public HolonRef ref;
        public String error = "";
    }

    public static final class ConnectResult {
        public Object channel;
        public String uid = "";
        public HolonRef origin;
        public String error = "";
    }
}
