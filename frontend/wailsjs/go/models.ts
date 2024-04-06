export namespace model {
	
	export class Config {
	    server_ip: string;
	    server_port: number;
	    peer_name: string;
	    group_name: string;
	    group_pwd: string;
	    crypt_type: number;
	    peer_pwd: string;
	    tab_name: string;
	    hw_mac: string;
	    ip_mode: number;
	    ip_addr: string;
	    ip_mask: string;
	    allow_visit_port: boolean;
	    enable_log: boolean;
	    log_level: number;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_ip = source["server_ip"];
	        this.server_port = source["server_port"];
	        this.peer_name = source["peer_name"];
	        this.group_name = source["group_name"];
	        this.group_pwd = source["group_pwd"];
	        this.crypt_type = source["crypt_type"];
	        this.peer_pwd = source["peer_pwd"];
	        this.tab_name = source["tab_name"];
	        this.hw_mac = source["hw_mac"];
	        this.ip_mode = source["ip_mode"];
	        this.ip_addr = source["ip_addr"];
	        this.ip_mask = source["ip_mask"];
	        this.allow_visit_port = source["allow_visit_port"];
	        this.enable_log = source["enable_log"];
	        this.log_level = source["log_level"];
	    }
	}
	export class PeerInfo {
	    peer_name?: string;
	    peer_mac?: string;
	    dev_type?: string;
	    net_addr?: string;
	    inter_addr?: string;
	    online: boolean;
	    link_mode: number;
	    link_quality: number;
	    connect_type: number;
	    total_rx_tx: string;
	    p2p_rx_tx: string;
	    trans_rx_tx: string;
	
	    static createFrom(source: any = {}) {
	        return new PeerInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.peer_name = source["peer_name"];
	        this.peer_mac = source["peer_mac"];
	        this.dev_type = source["dev_type"];
	        this.net_addr = source["net_addr"];
	        this.inter_addr = source["inter_addr"];
	        this.online = source["online"];
	        this.link_mode = source["link_mode"];
	        this.link_quality = source["link_quality"];
	        this.connect_type = source["connect_type"];
	        this.total_rx_tx = source["total_rx_tx"];
	        this.p2p_rx_tx = source["p2p_rx_tx"];
	        this.trans_rx_tx = source["trans_rx_tx"];
	    }
	}

}

export namespace protocol {
	
	export class LinkInfo {
	    addr?: number;
	    rx?: number;
	    tx?: number;
	
	    static createFrom(source: any = {}) {
	        return new LinkInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.addr = source["addr"];
	        this.rx = source["rx"];
	        this.tx = source["tx"];
	    }
	}
	export class Statistics {
	    trans_send?: number;
	    trans_receive?: number;
	    p2p_send?: number;
	    p2p_receive?: number;
	
	    static createFrom(source: any = {}) {
	        return new Statistics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.trans_send = source["trans_send"];
	        this.trans_receive = source["trans_receive"];
	        this.p2p_send = source["p2p_send"];
	        this.p2p_receive = source["p2p_receive"];
	    }
	}

}

