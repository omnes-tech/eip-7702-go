// SPDX-License-Identifier: MIT
pragma solidity ^0.8.17;

import {ERC20} from "@openzeppelin/contracts/token/ERC20/ERC20.sol";

contract Token is ERC20 {
    constructor() ERC20("EIP7702", "EIP7702") {}

    function mint(address to, uint256 amount) external {
        _mint(to, amount);
    }
}
