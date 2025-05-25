// SPDX-License-Identifier: MIT
pragma solidity ^0.8.17;

import {Script} from "forge-std/Script.sol";
import {Token} from "../src/Token.sol";
import {SimpleDelegateContract} from "../src/SimpleDelegateContract.sol";
import {console} from "forge-std/console.sol";

contract DeployEip7702 is Script {
    //Token public token;
    SimpleDelegateContract public delegateContract;

    struct deployConfig {
        uint256 deployerKey;
    }

    function run() public {
        deployConfig memory cfg = config();
        vm.startBroadcast(cfg.deployerKey);
        //token = new Token();

        // Pequeno delay simulado
        //vm.roll(block.number + 1);

        delegateContract = new SimpleDelegateContract();
        //console.log("Token deployed to:", address(token));
        console.log("Delegate contract deployed to:", address(delegateContract));
        vm.stopBroadcast();
    }

    function config() public returns (deployConfig memory) {
        return deployConfig({deployerKey: vm.envUint("PRIVATE_KEY")});
    }

    /*
    == Logs ==
    Token deployed to: 0x93d77bE58A977350B924C0694242b075eB26AEdE
    Delegate contract deployed to: 0x1f0F9d7e19991e7E296630DC0073610f23CF066a
    */
}
